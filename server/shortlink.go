package main

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/dgraph-io/badger/v4"
	"github.com/gorilla/mux"
)

const (
	shortSlugCharset        = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	shortSlugMinLen         = 5
	shortSlugMaxLen         = 12
	shortSlugAttemptsPerLen = 10
	recordKeyPrefix         = "r:"
)

// FileRecord is the durable, db-backed record for one uploaded file. Rows are
// never removed - deletion (and replacement on upload) only flips Deleted, so
// a slug can never be reused for different content.
type FileRecord struct {
	Slug        string `json:"slug"`
	DisplayName string `json:"display_name"`
	StorageKey  string `json:"storage_key"` // path within uploadFolder, e.g. "ab/cdefgh..."
	Size        int64  `json:"size"`
	Modified    string `json:"modified"`
	MimeType    string `json:"mime_type"`
	Deleted     bool   `json:"deleted"`
	// Pending is true between reserving a slug and finishing the write to
	// disk, so an in-flight upload never shows up as a live file.
	Pending bool `json:"pending"`
}

func recordKey(slug string) []byte {
	return []byte(recordKeyPrefix + slug)
}

// storageKeyForSlug shards files into subdirectories by slug prefix instead of
// storing them on disk under their (untrusted, possibly duplicated) display name.
func storageKeyForSlug(slug string) string {
	return slug[:2] + "/" + slug[2:]
}

// randomSlug generates a random slug using crypto/rand: slugs gate access to
// uploaded files, so they must not be predictable.
func randomSlug(length int) (string, error) {
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	slug := make([]byte, length)
	for i, b := range buf {
		slug[i] = shortSlugCharset[int(b)%len(shortSlugCharset)]
	}
	return string(slug), nil
}

// generateUniqueSlug finds a slug not already present in txn, growing the
// length if repeated collisions occur.
func generateUniqueSlug(txn *badger.Txn) (string, error) {
	for length := shortSlugMinLen; length <= shortSlugMaxLen; length++ {
		for attempt := 0; attempt < shortSlugAttemptsPerLen; attempt++ {
			slug, err := randomSlug(length)
			if err != nil {
				return "", err
			}
			_, err = txn.Get(recordKey(slug))
			if errors.Is(err, badger.ErrKeyNotFound) {
				return slug, nil
			}
			if err != nil {
				return "", err
			}
		}
	}
	return "", errors.New("failed to generate a unique short link slug")
}

func saveRecord(txn *badger.Txn, rec FileRecord) error {
	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return txn.Set(recordKey(rec.Slug), data)
}

func readRecord(txn *badger.Txn, slug string) (FileRecord, error) {
	var rec FileRecord
	item, err := txn.Get(recordKey(slug))
	if err != nil {
		return rec, err
	}
	err = item.Value(func(val []byte) error {
		return json.Unmarshal(val, &rec)
	})
	return rec, err
}

// getFileRecord returns the record for slug, regardless of its Deleted/Pending state.
func getFileRecord(slug string) (FileRecord, error) {
	var rec FileRecord
	err := shortLinkDB.View(func(txn *badger.Txn) error {
		var err error
		rec, err = readRecord(txn, slug)
		return err
	})
	return rec, err
}

// findLiveRecordByName returns the (at most one, unless append-only) live
// record with the given display name.
func findLiveRecordByName(txn *badger.Txn, displayName string) (*FileRecord, error) {
	it := txn.NewIterator(badger.DefaultIteratorOptions)
	defer it.Close()

	prefix := []byte(recordKeyPrefix)
	for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
		var rec FileRecord
		err := it.Item().Value(func(val []byte) error {
			return json.Unmarshal(val, &rec)
		})
		if err != nil {
			return nil, err
		}
		if !rec.Deleted && !rec.Pending && rec.DisplayName == displayName {
			return &rec, nil
		}
	}
	return nil, nil
}

// listLiveRecords returns every non-deleted, non-pending record: the db is
// the single source of truth for listing (no directory scan).
func listLiveRecords() ([]FileRecord, error) {
	var records []FileRecord
	err := shortLinkDB.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		prefix := []byte(recordKeyPrefix)
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			var rec FileRecord
			err := it.Item().Value(func(val []byte) error {
				return json.Unmarshal(val, &rec)
			})
			if err != nil {
				return err
			}
			if !rec.Deleted && !rec.Pending {
				records = append(records, rec)
			}
		}
		return nil
	})
	return records, err
}

// reserveUpload creates a new pending record for displayName and, unless
// appendOnly is set, soft-deletes the previous live record with the same
// name (returned so the caller can reclaim its physical blob once the new
// upload succeeds). Soft-deleted rows are kept forever so their slug is
// never reused.
func reserveUpload(displayName, mimeType string) (rec FileRecord, replaced *FileRecord, err error) {
	err = shortLinkDB.Update(func(txn *badger.Txn) error {
		slug, err := generateUniqueSlug(txn)
		if err != nil {
			return err
		}

		if !appendOnly {
			replaced, err = findLiveRecordByName(txn, displayName)
			if err != nil {
				return err
			}
			if replaced != nil {
				deleted := *replaced
				deleted.Deleted = true
				if err := saveRecord(txn, deleted); err != nil {
					return err
				}
			}
		}

		rec = FileRecord{
			Slug:        slug,
			DisplayName: displayName,
			StorageKey:  storageKeyForSlug(slug),
			MimeType:    mimeType,
			Pending:     true,
		}
		return saveRecord(txn, rec)
	})
	return rec, replaced, err
}

// finalizeUpload patches the record with its final size/modified time and
// clears Pending once the file has been fully written to disk.
func finalizeUpload(slug string, size int64, modified string) (FileRecord, error) {
	var rec FileRecord
	err := shortLinkDB.Update(func(txn *badger.Txn) error {
		var err error
		rec, err = readRecord(txn, slug)
		if err != nil {
			return err
		}
		rec.Size = size
		rec.Modified = modified
		rec.Pending = false
		return saveRecord(txn, rec)
	})
	return rec, err
}

// abortUpload marks a reserved record as deleted after a failed write so its
// slug is never reused, without ever leaving it visible as a live file.
func abortUpload(slug string) error {
	return shortLinkDB.Update(func(txn *badger.Txn) error {
		rec, err := readRecord(txn, slug)
		if err != nil {
			return err
		}
		rec.Deleted = true
		rec.Pending = false
		return saveRecord(txn, rec)
	})
}

// softDeleteByDisplayName marks the live record for displayName as deleted,
// keeping the row (and its slug) around forever.
func softDeleteByDisplayName(displayName string) (*FileRecord, error) {
	var rec *FileRecord
	err := shortLinkDB.Update(func(txn *badger.Txn) error {
		found, err := findLiveRecordByName(txn, displayName)
		if err != nil {
			return err
		}
		if found == nil {
			return nil
		}
		deleted := *found
		deleted.Deleted = true
		if err := saveRecord(txn, deleted); err != nil {
			return err
		}
		rec = &deleted
		return nil
	})
	return rec, err
}

func shortLinkHandler(w http.ResponseWriter, r *http.Request) {
	slug := mux.Vars(r)["slug"]

	rec, err := getFileRecord(slug)
	if errors.Is(err, badger.ErrKeyNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Failed to resolve short link", http.StatusInternalServerError)
		return
	}
	if rec.Deleted || rec.Pending {
		http.NotFound(w, r)
		return
	}

	serveFileRecord(w, r, rec)
}
