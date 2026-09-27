package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
)

// fsStorage keeps blobs as plain files under dir, in the slug-sharded layout
// ("ab/cdefgh...") they have always used. This is the default backend.
type fsStorage struct {
	dir string
}

func newFsStorage(dir string) *fsStorage {
	return &fsStorage{dir: dir}
}

func (s *fsStorage) path(key string) string {
	return filepath.Join(s.dir, key)
}

func (s *fsStorage) Put(ctx context.Context, key string, r io.Reader, maxBytes int64) (int64, error) {
	dest := s.path(key)
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return 0, err
	}

	dst, err := os.Create(dest)
	if err != nil {
		return 0, err
	}

	reader := newCapReader(r, maxBytes)
	buffer := make([]byte, chunkSize)
	written, err := io.CopyBuffer(dst, reader, buffer)
	if err != nil {
		dst.Close()
		os.Remove(dest)
		if reader.overflowed {
			return 0, errBlobTooLarge
		}
		return 0, err
	}
	if written > maxBytes {
		dst.Close()
		os.Remove(dest)
		return 0, errBlobTooLarge
	}
	// A failed Close can mean the last write never reached the disk, so the
	// blob must not be treated as stored.
	if err := dst.Close(); err != nil {
		os.Remove(dest)
		return 0, err
	}

	return written, nil
}

func (s *fsStorage) Get(ctx context.Context, key string) (io.ReadSeekCloser, BlobInfo, error) {
	file, err := os.Open(s.path(key))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, BlobInfo{}, errBlobNotFound
		}
		return nil, BlobInfo{}, err
	}

	stat, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, BlobInfo{}, err
	}
	if stat.IsDir() {
		file.Close()
		return nil, BlobInfo{}, errBlobNotFound
	}

	return file, BlobInfo{Size: stat.Size(), ModTime: stat.ModTime()}, nil
}

func (s *fsStorage) Delete(ctx context.Context, key string) error {
	if err := os.Remove(s.path(key)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
