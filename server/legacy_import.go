package main

import (
	"log"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// importLegacyUploads is a one-time migration, run only when the sqlite db
// didn't already exist: any flat files already sitting in uploadFolder (from
// before the db-backed model existed) are registered as records and moved
// into their slug-sharded storage path. The db file itself (and its -wal/-shm
// sidecars, now that it lives in uploadFolder too) are skipped.
func importLegacyUploads() error {
	entries, err := os.ReadDir(uploadFolder)
	if err != nil {
		return err
	}

	imported := 0
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), dbFileName) {
			continue
		}
		if err := importLegacyUpload(entry.Name()); err != nil {
			log.Printf("Failed to import legacy file %s: %v", entry.Name(), err)
			continue
		}
		imported++
	}
	if imported > 0 {
		log.Printf("Imported %d legacy file(s) from %s into the file record database", imported, uploadFolder)
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

	rec, err := insertUniqueRecord(appDB, FileRecord{
		DisplayName: displayName,
		Size:        stat.Size(),
		Modified:    stat.ModTime().UTC().Format(time.RFC3339),
		MimeType:    mimeType,
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
