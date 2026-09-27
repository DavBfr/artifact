package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

func TestFsStorageRoundTrip(t *testing.T) {
	store := newFsStorage(t.TempDir())
	ctx := context.Background()
	content := []byte("hello artifact server")

	written, err := store.Put(ctx, "ab/cdef", bytes.NewReader(content), 1024)
	if err != nil {
		t.Fatalf("Put returned unexpected error: %v", err)
	}
	if written != int64(len(content)) {
		t.Fatalf("Put stored %d bytes, want %d", written, len(content))
	}

	blob, info, err := store.Get(ctx, "ab/cdef")
	if err != nil {
		t.Fatalf("Get returned unexpected error: %v", err)
	}
	defer blob.Close()

	if info.Size != int64(len(content)) {
		t.Fatalf("Get reported size %d, want %d", info.Size, len(content))
	}
	got, err := io.ReadAll(blob)
	if err != nil {
		t.Fatalf("reading the blob failed: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("Get returned %q, want %q", got, content)
	}

	if err := store.Delete(ctx, "ab/cdef"); err != nil {
		t.Fatalf("Delete returned unexpected error: %v", err)
	}
	if _, _, err := store.Get(ctx, "ab/cdef"); !errors.Is(err, errBlobNotFound) {
		t.Fatalf("Get after Delete returned %v, want errBlobNotFound", err)
	}
	// Deleting an absent blob has to stay idempotent so replaced-file cleanup
	// and retries can't fail on it.
	if err := store.Delete(ctx, "ab/cdef"); err != nil {
		t.Fatalf("second Delete returned unexpected error: %v", err)
	}
}

func TestFsStorageMissingBlob(t *testing.T) {
	store := newFsStorage(t.TempDir())
	if _, _, err := store.Get(context.Background(), "ab/nope"); !errors.Is(err, errBlobNotFound) {
		t.Fatalf("Get on a missing key returned %v, want errBlobNotFound", err)
	}
}

func TestFsStorageRejectsOversizedBlobs(t *testing.T) {
	store := newFsStorage(t.TempDir())
	ctx := context.Background()

	tests := []struct {
		name    string
		size    int64
		wantErr bool
	}{
		{name: "at the cap", size: 10},
		{name: "one over the cap", size: 11, wantErr: true},
		{name: "far over the cap", size: 4096, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			key := fmt.Sprintf("ab/%d", tc.size)
			_, err := store.Put(ctx, key, bytes.NewReader(make([]byte, tc.size)), 10)

			if !tc.wantErr {
				if err != nil {
					t.Fatalf("Put(%d bytes, cap 10) returned unexpected error: %v", tc.size, err)
				}
				return
			}

			if !errors.Is(err, errBlobTooLarge) {
				t.Fatalf("Put(%d bytes, cap 10) returned %v, want errBlobTooLarge", tc.size, err)
			}
			// Nothing may be left behind: a surviving partial blob would later
			// be served as if it were the whole file.
			if _, _, err := store.Get(ctx, key); !errors.Is(err, errBlobNotFound) {
				t.Fatalf("oversized Put left a blob behind (Get returned %v)", err)
			}
		})
	}
}

// eofReader hands back all of its bytes together with io.EOF, the shape that
// defeats a size check based purely on the reader failing.
type eofReader struct{ data []byte }

func (r *eofReader) Read(p []byte) (int, error) {
	n := copy(p, r.data)
	r.data = r.data[n:]
	if len(r.data) == 0 {
		return n, io.EOF
	}
	return n, nil
}

func TestFsStorageCatchesOverflowReportedWithEOF(t *testing.T) {
	store := newFsStorage(t.TempDir())
	_, err := store.Put(context.Background(), "ab/boundary", &eofReader{data: make([]byte, 11)}, 10)
	if !errors.Is(err, errBlobTooLarge) {
		t.Fatalf("Put returned %v, want errBlobTooLarge", err)
	}
}

// fakeS3 is an in-memory stand-in for the SDK client, which keeps the ranged
// reader and the size cap testable without a network.
type fakeS3 struct {
	objects   map[string][]byte
	gets      int
	lastRange string
	modTime   time.Time
}

func newFakeS3() *fakeS3 {
	return &fakeS3{
		objects: map[string][]byte{},
		modTime: time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC),
	}
}

func (f *fakeS3) HeadObject(ctx context.Context, in *s3.HeadObjectInput, opt ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	data, ok := f.objects[aws.ToString(in.Key)]
	if !ok {
		return nil, &types.NotFound{}
	}
	return &s3.HeadObjectOutput{
		ContentLength: aws.Int64(int64(len(data))),
		LastModified:  aws.Time(f.modTime),
	}, nil
}

func (f *fakeS3) GetObject(ctx context.Context, in *s3.GetObjectInput, opt ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	data, ok := f.objects[aws.ToString(in.Key)]
	if !ok {
		return nil, &types.NoSuchKey{}
	}

	f.gets++
	f.lastRange = aws.ToString(in.Range)

	start := 0
	if r := aws.ToString(in.Range); r != "" {
		if _, err := fmt.Sscanf(r, "bytes=%d-", &start); err != nil {
			return nil, fmt.Errorf("fake s3: unparsable range %q", r)
		}
	}
	if start > len(data) {
		start = len(data)
	}

	return &s3.GetObjectOutput{Body: io.NopCloser(bytes.NewReader(data[start:]))}, nil
}

func (f *fakeS3) DeleteObject(ctx context.Context, in *s3.DeleteObjectInput, opt ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	delete(f.objects, aws.ToString(in.Key))
	return &s3.DeleteObjectOutput{}, nil
}

// fakeUploader stands in for the transfer manager: for a single-object upload
// that amounts to reading the body and storing it under the key.
type fakeUploader struct{ store *fakeS3 }

func (f *fakeUploader) UploadObject(ctx context.Context, in *transfermanager.UploadObjectInput, opt ...func(*transfermanager.Options)) (*transfermanager.UploadObjectOutput, error) {
	data, err := io.ReadAll(in.Body)
	if err != nil {
		return nil, err
	}

	// The in-process fake is only ever driven from the test goroutine, so it
	// needs no locking (unlike the HTTP stub).
	f.store.objects[aws.ToString(in.Key)] = data

	return &transfermanager.UploadObjectOutput{}, nil
}

// newFakeBackedStorage returns a store backed by the in-memory fake, upload
// path included.
func newFakeBackedStorage() (*s3Storage, *fakeS3) {
	fake := newFakeS3()
	return &s3Storage{
		client:   fake,
		uploader: &fakeUploader{store: fake},
		bucket:   "bucket",
		prefix:   "artifacts/",
	}, fake
}

func TestS3StorageRoundTrip(t *testing.T) {
	store, fake := newFakeBackedStorage()
	ctx := context.Background()
	content := []byte("0123456789")

	if _, err := store.Put(ctx, "ab/cdef", bytes.NewReader(content), 1024); err != nil {
		t.Fatalf("Put returned unexpected error: %v", err)
	}
	if got := fake.objects["artifacts/ab/cdef"]; !bytes.Equal(got, content) {
		t.Fatalf("Put stored %q under artifacts/ab/cdef", got)
	}

	blob, info, err := store.Get(ctx, "ab/cdef")
	if err != nil {
		t.Fatalf("Get returned unexpected error: %v", err)
	}
	defer blob.Close()

	if info.Size != int64(len(content)) {
		t.Fatalf("Get reported size %d, want %d", info.Size, len(content))
	}
	// Sizing a response must not cost a transfer: the size comes from the HEAD.
	if fake.gets != 0 {
		t.Fatalf("Get issued %d GET request(s) before anything was read", fake.gets)
	}

	got, err := io.ReadAll(blob)
	if err != nil {
		t.Fatalf("reading the blob failed: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("Get returned %q, want %q", got, content)
	}
	// A straight read through should be a single request on the whole object.
	if fake.gets != 1 {
		t.Fatalf("reading the whole blob issued %d GET request(s), want 1", fake.gets)
	}
	if fake.lastRange != "" {
		t.Fatalf("the first request asked for range %q, want the whole object", fake.lastRange)
	}

	if err := store.Delete(ctx, "ab/cdef"); err != nil {
		t.Fatalf("Delete returned unexpected error: %v", err)
	}
	if _, _, err := store.Get(ctx, "ab/cdef"); !errors.Is(err, errBlobNotFound) {
		t.Fatalf("Get after Delete returned %v, want errBlobNotFound", err)
	}
}

func TestS3StorageRejectsOversizedBlobs(t *testing.T) {
	store, fake := newFakeBackedStorage()

	_, err := store.Put(context.Background(), "ab/big", bytes.NewReader(make([]byte, 50)), 10)
	if !errors.Is(err, errBlobTooLarge) {
		t.Fatalf("Put returned %v, want errBlobTooLarge", err)
	}
	if len(fake.objects) != 0 {
		t.Fatalf("oversized Put left %d object(s) behind", len(fake.objects))
	}
}

func TestS3ReadSeekerServesRanges(t *testing.T) {
	store, fake := newFakeBackedStorage()
	ctx := context.Background()
	content := []byte("abcdefghij")

	if _, err := store.Put(ctx, "ab/ranged", bytes.NewReader(content), 1024); err != nil {
		t.Fatalf("Put returned unexpected error: %v", err)
	}

	blob, _, err := store.Get(ctx, "ab/ranged")
	if err != nil {
		t.Fatalf("Get returned unexpected error: %v", err)
	}
	defer blob.Close()

	// Seek to the end is how the response size is discovered; it must be
	// answered from the HEAD result rather than by fetching anything.
	if pos, err := blob.Seek(0, io.SeekEnd); err != nil || pos != int64(len(content)) {
		t.Fatalf("Seek(0, io.SeekEnd) = (%d, %v), want (%d, nil)", pos, err, len(content))
	}
	if fake.gets != 0 {
		t.Fatalf("seeking issued %d GET request(s)", fake.gets)
	}

	// A seek forward then a read must fetch only the tail.
	if pos, err := blob.Seek(7, io.SeekStart); err != nil || pos != 7 {
		t.Fatalf("Seek(7, io.SeekStart) = (%d, %v), want (7, nil)", pos, err)
	}
	tail, err := io.ReadAll(blob)
	if err != nil {
		t.Fatalf("reading from offset 7 failed: %v", err)
	}
	if string(tail) != "hij" {
		t.Fatalf("read from offset 7 = %q, want %q", tail, "hij")
	}
	if fake.lastRange != "bytes=7-" {
		t.Fatalf("the request used range %q, want %q", fake.lastRange, "bytes=7-")
	}

	// Seeking back to the start reopens the whole object.
	if _, err := blob.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("Seek(0, io.SeekStart) failed: %v", err)
	}
	all, err := io.ReadAll(blob)
	if err != nil {
		t.Fatalf("reading from the start failed: %v", err)
	}
	if !bytes.Equal(all, content) {
		t.Fatalf("read from the start = %q, want %q", all, content)
	}
}

// TestS3ReadSeekerWithServeContent pins down the contract serveFileRecord
// relies on: http.ServeContent must be able to size, seek and range over an
// object without downloading anything it wasn't asked for.
func TestS3ReadSeekerWithServeContent(t *testing.T) {
	store, fake := newFakeBackedStorage()
	ctx := context.Background()
	content := []byte("abcdefghij")

	if _, err := store.Put(ctx, "ab/http", bytes.NewReader(content), 1024); err != nil {
		t.Fatalf("Put returned unexpected error: %v", err)
	}

	t.Run("ranged request", func(t *testing.T) {
		blob, info, err := store.Get(ctx, "ab/http")
		if err != nil {
			t.Fatalf("Get returned unexpected error: %v", err)
		}
		defer blob.Close()

		req := httptest.NewRequest(http.MethodGet, "/s/ab/http", nil)
		req.Header.Set("Range", "bytes=2-5")
		rec := httptest.NewRecorder()
		http.ServeContent(rec, req, "file.txt", info.ModTime, blob)

		if rec.Code != http.StatusPartialContent {
			t.Fatalf("ranged ServeContent returned %d, want 206", rec.Code)
		}
		if got := rec.Body.String(); got != "cdef" {
			t.Fatalf("ranged body = %q, want %q", got, "cdef")
		}
		if got := rec.Header().Get("Content-Range"); got != "bytes 2-5/10" {
			t.Fatalf("Content-Range = %q, want %q", got, "bytes 2-5/10")
		}
	})

	t.Run("whole object", func(t *testing.T) {
		before := fake.gets

		blob, info, err := store.Get(ctx, "ab/http")
		if err != nil {
			t.Fatalf("Get returned unexpected error: %v", err)
		}
		defer blob.Close()

		req := httptest.NewRequest(http.MethodGet, "/s/ab/http", nil)
		rec := httptest.NewRecorder()
		http.ServeContent(rec, req, "file.txt", info.ModTime, blob)

		if rec.Code != http.StatusOK {
			t.Fatalf("ServeContent returned %d, want 200", rec.Code)
		}
		if !bytes.Equal(rec.Body.Bytes(), content) {
			t.Fatalf("body = %q, want %q", rec.Body.Bytes(), content)
		}
		if got := fake.gets - before; got != 1 {
			t.Fatalf("serving the whole object took %d GET request(s), want 1", got)
		}
	})
}

func TestIsS3NotFound(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "typed NotFound", err: &types.NotFound{}, want: true},
		{name: "typed NoSuchKey", err: &types.NoSuchKey{}, want: true},
		{name: "generic API error", err: &smithy.GenericAPIError{Code: "NoSuchKey"}, want: true},
		{name: "generic 404 code", err: &smithy.GenericAPIError{Code: "404"}, want: true},
		{name: "other API error", err: &smithy.GenericAPIError{Code: "AccessDenied"}, want: false},
		{name: "unrelated error", err: errors.New("connection reset"), want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isS3NotFound(fmt.Errorf("wrapped: %w", tc.err)); got != tc.want {
				t.Fatalf("isS3NotFound(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestNormaliseS3Prefix(t *testing.T) {
	tests := map[string]string{
		"":              "",
		"artifacts":     "artifacts/",
		"artifacts/":    "artifacts/",
		"/artifacts":    "artifacts/",
		"/a/b/":         "a/b/",
		"  artifacts  ": "artifacts/",
	}

	for in, want := range tests {
		if got := normaliseS3Prefix(in); got != want {
			t.Fatalf("normaliseS3Prefix(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCapReaderStopsAtTheCap(t *testing.T) {
	reader := newCapReader(strings.NewReader("0123456789"), 4)

	// Exactly one byte past the cap is allowed through, which is what tells
	// "exactly at the cap" apart from "over it"; the next read fails.
	got, err := io.ReadAll(reader)
	if !errors.Is(err, errBlobTooLarge) {
		t.Fatalf("ReadAll returned %v, want errBlobTooLarge", err)
	}
	if string(got) != "01234" {
		t.Fatalf("capReader passed %q through, want %q", got, "01234")
	}
	if !reader.overflowed {
		t.Fatal("capReader did not record the overflow")
	}
}
