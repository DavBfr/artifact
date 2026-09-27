package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// stubS3 is a minimal S3 endpoint, enough of the protocol to exercise the real
// SDK client over HTTP. The in-process fake in storage_test.go checks this
// package's logic; this checks the part that can't be checked any other way
// offline: how the SDK frames a streaming upload and ranged reads on the wire.
type stubS3 struct {
	mu      sync.Mutex
	objects map[string][]byte
	putHead http.Header
}

func newStubS3() *stubS3 {
	return &stubS3{objects: map[string][]byte{}}
}

func (s *stubS3) object(key string) []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.objects[key]
}

func (s *stubS3) objectCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.objects)
}

func (s *stubS3) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Path-style addressing: /<bucket>/<key...>
	path := strings.TrimPrefix(r.URL.Path, "/")
	bucket, key, _ := strings.Cut(path, "/")
	if bucket == "" || key == "" {
		http.Error(w, "stub: need /bucket/key", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	switch r.Method {
	case http.MethodPut:
		s.putHead = r.Header.Clone()
		body, err := readS3Body(r)
		if err != nil {
			http.Error(w, "stub: "+err.Error(), http.StatusInternalServerError)
			return
		}
		s.objects[path] = body
		w.Header().Set("ETag", `"stub-etag"`)
		w.WriteHeader(http.StatusOK)

	case http.MethodHead:
		data, ok := s.objects[path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
		w.WriteHeader(http.StatusOK)

	case http.MethodGet:
		data, ok := s.objects[path]
		if !ok {
			writeStubS3Error(w, "NoSuchKey")
			return
		}

		start := 0
		if rng := r.Header.Get("Range"); rng != "" {
			if _, err := fmt.Sscanf(rng, "bytes=%d-", &start); err != nil || start < 0 || start > len(data) {
				http.Error(w, "stub: bad range "+rng, http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, len(data)-1, len(data)))
			w.WriteHeader(http.StatusPartialContent)
			w.Write(data[start:])
			return
		}

		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.WriteHeader(http.StatusOK)
		w.Write(data)

	case http.MethodDelete:
		delete(s.objects, path)
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "stub: unsupported method", http.StatusMethodNotAllowed)
	}
}

// readS3Body returns the object body, unwrapping the SigV4 streaming framing
// the SDK uses when it doesn't know the length up front:
//
//	<hex-size>;chunk-signature=…\r\n<data>\r\n … 0;chunk-signature=…\r\n
func readS3Body(r *http.Request) ([]byte, error) {
	if !strings.Contains(r.Header.Get("Content-Encoding"), "aws-chunked") {
		return io.ReadAll(r.Body)
	}

	reader := bufio.NewReader(r.Body)
	var out bytes.Buffer
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("reading chunk header: %w", err)
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			continue
		}

		sizeField := line
		if i := strings.IndexByte(line, ';'); i >= 0 {
			sizeField = line[:i]
		}
		size, err := strconv.ParseInt(strings.TrimSpace(sizeField), 16, 64)
		if err != nil {
			return nil, fmt.Errorf("unparsable chunk size %q: %w", sizeField, err)
		}
		if size == 0 {
			// Final chunk; any trailing checksum trailer is ignored.
			return out.Bytes(), nil
		}
		if _, err := io.CopyN(&out, reader, size); err != nil {
			return nil, fmt.Errorf("reading chunk body: %w", err)
		}
		if _, err := reader.Discard(2); err != nil { // CRLF after the data
			return nil, err
		}
	}
}

func writeStubS3Error(w http.ResponseWriter, code string) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusNotFound)
	fmt.Fprintf(w,
		`<?xml version="1.0" encoding="UTF-8"?><Error><Code>%s</Code><Message>%s</Message><RequestId>stub</RequestId></Error>`,
		code, code)
}

// newStubBackedStorage returns an s3Storage pointed at an in-process S3 stub,
// built with the same client options as production, going through the real SDK
// client (including SigV4 signing).
func newStubBackedStorage(t *testing.T) (*s3Storage, *stubS3) {
	return newStubBackedStorageWith(t, false)
}

// newStubBackedStorageWith picks the transport deliberately: over TLS the SDK
// protects a streamed upload with a trailing checksum, while over plain HTTP it
// cannot and falls back to the relaxed path - the internal-MinIO case.
func newStubBackedStorageWith(t *testing.T, overTLS bool) (*s3Storage, *stubS3) {
	t.Helper()

	stub := newStubS3()
	server := httptest.NewServer(stub)
	if overTLS {
		server = httptest.NewTLSServer(stub)
	}
	t.Cleanup(server.Close)

	cfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion("us-east-1"),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider("testkey", "testsecret", ""),
		),
	)
	if err != nil {
		t.Fatalf("building aws config: %v", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		s3ClientOptions(server.URL, true)(o)
		if overTLS {
			// The TLS stub signs its own certificate, so it hands over its
			// client rather than an insecure one being configured here.
			o.HTTPClient = server.Client()
		}
	})

	return &s3Storage{
		client:   client,
		uploader: transfermanager.New(client),
		bucket:   "bucket",
		prefix:   "artifacts/",
	}, stub
}

func TestS3BackendAgainstStubEndpoint(t *testing.T) {
	for _, overTLS := range []bool{false, true} {
		transport := "http"
		if overTLS {
			transport = "https"
		}

		t.Run(transport, func(t *testing.T) {
			store, stub := newStubBackedStorageWith(t, overTLS)
			ctx := context.Background()
			content := []byte("the quick brown fox jumps over the lazy dog")

			written, err := store.Put(ctx, "ab/streamy", bytes.NewReader(content), 1<<20)
			if err != nil {
				t.Fatalf("Put against the stub endpoint failed: %v", err)
			}
			if written != int64(len(content)) {
				t.Fatalf("Put reported %d bytes, want %d", written, len(content))
			}

			// Report the framing so the shape of a streaming upload is visible in
			// the test output rather than assumed.
			t.Logf("PUT over %s: Content-Length=%q Content-Encoding=%q Transfer-Encoding=%q X-Amz-Content-Sha256=%q",
				transport,
				stub.putHead.Get("Content-Length"),
				stub.putHead.Get("Content-Encoding"),
				stub.putHead.Get("Transfer-Encoding"),
				stub.putHead.Get("X-Amz-Content-Sha256"),
			)

			// The stub stores what it decoded from the wire, so a mismatch means
			// the framing wasn't understood - i.e. the upload lost its bytes.
			if got := stub.object("bucket/artifacts/ab/streamy"); !bytes.Equal(got, content) {
				t.Fatalf("the endpoint received %q, want %q", got, content)
			}

			blob, info, err := store.Get(ctx, "ab/streamy")
			if err != nil {
				t.Fatalf("Get against the stub endpoint failed: %v", err)
			}
			defer blob.Close()

			if info.Size != int64(len(content)) {
				t.Fatalf("Get reported size %d, want %d", info.Size, len(content))
			}
			if info.ModTime.IsZero() {
				t.Fatal("Get reported a zero modification time")
			}

			got, err := io.ReadAll(blob)
			if err != nil {
				t.Fatalf("reading the blob failed: %v", err)
			}
			if !bytes.Equal(got, content) {
				t.Fatalf("read %q, want %q", got, content)
			}

			// A seek must turn into a ranged request the endpoint honours.
			if _, err := blob.Seek(4, io.SeekStart); err != nil {
				t.Fatalf("Seek failed: %v", err)
			}
			tail, err := io.ReadAll(blob)
			if err != nil {
				t.Fatalf("reading from an offset failed: %v", err)
			}
			if !bytes.Equal(tail, content[4:]) {
				t.Fatalf("read from offset 4 = %q, want %q", tail, content[4:])
			}

			if err := store.Delete(ctx, "ab/streamy"); err != nil {
				t.Fatalf("Delete against the stub endpoint failed: %v", err)
			}
			if _, _, err := store.Get(ctx, "ab/streamy"); !errors.Is(err, errBlobNotFound) {
				t.Fatalf("Get after Delete returned %v, want errBlobNotFound", err)
			}
		})
	}
}

func TestS3BackendStubRejectsOversizedBlobs(t *testing.T) {
	store, stub := newStubBackedStorage(t)

	_, err := store.Put(context.Background(), "ab/big", bytes.NewReader(make([]byte, 4096)), 32)
	if !errors.Is(err, errBlobTooLarge) {
		t.Fatalf("Put returned %v, want errBlobTooLarge", err)
	}
	if n := stub.objectCount(); n != 0 {
		t.Fatalf("oversized Put left %d object(s) behind", n)
	}
}

func TestS3BackendStubServesHTTPRanges(t *testing.T) {
	store, _ := newStubBackedStorage(t)
	ctx := context.Background()
	content := []byte("abcdefghij")

	if _, err := store.Put(ctx, "ab/http", bytes.NewReader(content), 1<<20); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	blob, info, err := store.Get(ctx, "ab/http")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	defer blob.Close()

	req := httptest.NewRequest(http.MethodGet, "/s/ab/http", nil)
	req.Header.Set("Range", "bytes=3-6")
	rec := httptest.NewRecorder()
	http.ServeContent(rec, req, "file.txt", info.ModTime, blob)

	if rec.Code != http.StatusPartialContent {
		t.Fatalf("ranged download returned %d, want 206", rec.Code)
	}
	if got := rec.Body.String(); got != "defg" {
		t.Fatalf("ranged body = %q, want %q", got, "defg")
	}
}
