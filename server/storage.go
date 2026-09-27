package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"
)

// storageKind selects which blob store backs uploaded files.
const (
	storageFilesystem = "fs"
	storageS3         = "s3"
)

// The two storage failures callers translate into HTTP responses (413 and 404).
// Every other error is a 500.
var (
	errBlobTooLarge = errors.New("blob exceeds the maximum size")
	errBlobNotFound = errors.New("blob not found")
)

// BlobInfo describes a stored blob without its contents.
type BlobInfo struct {
	Size    int64
	ModTime time.Time
	// ETag is the object store's own version tag for the blob. It is what lets a
	// reader tell "same object" from "new object" from a HEAD alone, since the
	// store derives it from the content: equal bytes give an equal tag. Empty
	// when the backend does not provide one.
	ETag string
}

// Storage is the blob store behind every uploaded file. Keys are always a
// FileRecord's StorageKey (slug-sharded, e.g. "ab/cdefgh..."), never a
// user-supplied name, so implementations can treat them as opaque.
type Storage interface {
	// Put streams r into key and reports how many bytes were stored, failing
	// with errBlobTooLarge once more than maxBytes are readable. A failed Put
	// leaves nothing behind under key.
	Put(ctx context.Context, key string, r io.Reader, maxBytes int64) (int64, error)
	// Get opens key for reading, returning errBlobNotFound when it is absent
	// (or is not a regular blob). The reader supports seeking so byte ranges
	// can be served.
	Get(ctx context.Context, key string) (io.ReadSeekCloser, BlobInfo, error)
	// Delete removes key. Deleting an absent key is not an error, which keeps
	// replaced-file cleanup idempotent.
	Delete(ctx context.Context, key string) error
}

// newStorage builds the blob store selected by ART_STORAGE.
func newStorage(ctx context.Context) (Storage, error) {
	switch storageKind {
	case storageFilesystem:
		return newFsStorage(uploadFolder), nil
	case storageS3:
		return newS3Storage(ctx)
	default:
		// Unreachable: main validates the value during startup.
		return nil, fmt.Errorf("unknown storage kind %q", storageKind)
	}
}

// initStorage builds the configured blob store into the package-level storage
// variable, giving it a bounded window to resolve credentials (an unreachable
// IMDS endpoint would otherwise hang startup).
func initStorage() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	s, err := newStorage(ctx)
	if err != nil {
		return err
	}
	storage = s
	return nil
}

// capReader passes at most maxBytes through and then fails with
// errBlobTooLarge, so an oversized upload is rejected while it streams rather
// than after the fact. It is allowed to read one byte past the cap, which is
// what distinguishes a stream of exactly maxBytes from an oversized one; the
// post-copy check for n > maxBytes catches the case where the source reports
// EOF on the same read that crossed the cap.
type capReader struct {
	r          io.Reader
	left       int64
	n          int64
	overflowed bool
}

func newCapReader(r io.Reader, maxBytes int64) *capReader {
	return &capReader{r: r, left: maxBytes + 1}
}

func (c *capReader) Read(p []byte) (int, error) {
	if c.left <= 0 {
		c.overflowed = true
		return 0, errBlobTooLarge
	}
	if int64(len(p)) > c.left {
		p = p[:c.left]
	}
	n, err := c.r.Read(p)
	c.left -= int64(n)
	c.n += int64(n)
	return n, err
}
