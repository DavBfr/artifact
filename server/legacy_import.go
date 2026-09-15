package main

import (
	"log"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dgraph-io/badger/v4"
)

// hasAnyRecords reports whether the db already holds at least one record
// (live or deleted), used to detect a fresh db that predates this feature.
func hasAnyRecords() (bool, error) {
	found := false
	err := shortLinkDB.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		defer it.Close()

		prefix := []byte(recordKeyPrefix)
		it.Seek(prefix)
		found = it.ValidForPrefix(prefix)
		return nil
	})
	return found, err
}

// importLegacyUploads is a one-time migration: if the db has no records yet,
// any flat files already sitting in uploadFolder (from before the db-backed
// model existed) are registered as records and moved into their slug-sharded
// storage path.
func importLegacyUploads() error {
	hasRecords, err := hasAnyRecords()
	if err != nil {
		return err
	}
	if hasRecords {
		return nil
	}

	entries, err := os.ReadDir(uploadFolder)
	if err != nil {
		return err
	}

	imported := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if err := importLegacyUpload(entry.Name()); err != nil {
			log.Printf("Failed to import legacy file %s: %v", entry.Name(), err)
			continue
		}
		imported++
	}
	if imported > 0 {
		log.Printf("Imported %d legacy file(s) from %s into the short link database", imported, uploadFolder)
	}
	return nil
}

// importLegacyUpload registers a single legacy file as a FileRecord and moves
// it from uploadFolder/<name> to its new slug-sharded storage path.
func importLegacyUpload(displayName string) error {
	oldPath := filepath.Join(uploadFolder, displayName)
	stat, err := os.Stat(oldPath)
	if err != nil {
		return err
	}

	var mimeType string
	if ext := strings.ToLower(filepath.Ext(displayName)); ext != "" {
		mimeType = mime.TypeByExtension(ext)
	}

	var rec FileRecord
	err = shortLinkDB.Update(func(txn *badger.Txn) error {
		slug, err := generateUniqueSlug(txn)
		if err != nil {
			return err
		}
		rec = FileRecord{
			Slug:        slug,
			DisplayName: displayName,
			StorageKey:  storageKeyForSlug(slug),
			Size:        stat.Size(),
			Modified:    stat.ModTime().UTC().Format(time.RFC3339),
			MimeType:    mimeType,
		}
		return saveRecord(txn, rec)
	})
	if err != nil {
		return err
	}

	newPath := filepath.Join(uploadFolder, rec.StorageKey)
	if err := os.MkdirAll(filepath.Dir(newPath), 0755); err != nil {
		_ = abortUpload(rec.Slug)
		return err
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		_ = abortUpload(rec.Slug)
		return err
	}
	return nil
}
