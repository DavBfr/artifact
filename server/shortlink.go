package main

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
)

const (
	shortSlugCharset        = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	shortSlugMinLen         = 5
	shortSlugMaxLen         = 12
	shortSlugAttemptsPerLen = 10
	recordColumns           = "slug, display_name, storage_key, size, modified, mime_type, deleted, pending"
)

var errRecordNotFound = errors.New("record not found")

// FileRecord is the durable, db-backed record for one uploaded file. Rows are
// never removed - deletion (and replacement on upload) only flips Deleted, so
// a slug can never be reused for different content.
type FileRecord struct {
	Slug        string
	DisplayName string
	StorageKey  string // path within uploadFolder, e.g. "ab/cdefgh..."
	Size        int64
	Modified    string
	MimeType    string
	Deleted     bool
	// Pending is true between reserving a slug and finishing the write to
	// disk, so an in-flight upload never shows up as a live file.
	Pending bool
}

// initSchema creates the files table (and its indexes) if they don't exist yet.
func initSchema() error {
	_, err := appDB.Exec(`
		CREATE TABLE IF NOT EXISTS files (
			slug         TEXT PRIMARY KEY,
			display_name TEXT NOT NULL,
			storage_key  TEXT NOT NULL,
			size         INTEGER NOT NULL DEFAULT 0,
			modified     TEXT NOT NULL DEFAULT '',
			mime_type    TEXT NOT NULL DEFAULT '',
			deleted      INTEGER NOT NULL DEFAULT 0,
			pending      INTEGER NOT NULL DEFAULT 0
		);
		CREATE INDEX IF NOT EXISTS idx_files_display_name ON files(display_name);
		CREATE INDEX IF NOT EXISTS idx_files_live ON files(deleted, pending);
	`)
	return err
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

// isUniqueConstraintErr reports whether err is a sqlite UNIQUE constraint violation.
func isUniqueConstraintErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

// execer is satisfied by both *sql.DB and *sql.Tx.
type execer interface {
	Exec(query string, args ...any) (sql.Result, error)
	QueryRow(query string, args ...any) *sql.Row
}

// scanner is satisfied by both *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...any) error
}

func scanRecord(s scanner) (FileRecord, error) {
	var rec FileRecord
	var deleted, pending int
	err := s.Scan(&rec.Slug, &rec.DisplayName, &rec.StorageKey, &rec.Size, &rec.Modified, &rec.MimeType, &deleted, &pending)
	if err != nil {
		return FileRecord{}, err
	}
	rec.Deleted = deleted != 0
	rec.Pending = pending != 0
	return rec, nil
}

// insertUniqueRecord inserts rec as a new row, generating a unique slug (and
// its derived storage key) and retrying with a longer slug on collision.
func insertUniqueRecord(db execer, rec FileRecord) (FileRecord, error) {
	for length := shortSlugMinLen; length <= shortSlugMaxLen; length++ {
		for attempt := 0; attempt < shortSlugAttemptsPerLen; attempt++ {
			slug, err := randomSlug(length)
			if err != nil {
				return FileRecord{}, err
			}
			rec.Slug = slug
			rec.StorageKey = storageKeyForSlug(slug)

			deleted, pending := 0, 0
			if rec.Deleted {
				deleted = 1
			}
			if rec.Pending {
				pending = 1
			}
			_, err = db.Exec(
				"INSERT INTO files ("+recordColumns+") VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
				rec.Slug, rec.DisplayName, rec.StorageKey, rec.Size, rec.Modified, rec.MimeType, deleted, pending,
			)
			if err == nil {
				return rec, nil
			}
			if !isUniqueConstraintErr(err) {
				return FileRecord{}, err
			}
			// slug collision: retry, growing the length after enough attempts
		}
	}
	return FileRecord{}, errors.New("failed to generate a unique short link slug")
}

// getFileRecord returns the record for slug, regardless of its Deleted/Pending state.
func getFileRecord(slug string) (FileRecord, error) {
	row := appDB.QueryRow("SELECT "+recordColumns+" FROM files WHERE slug = ?", slug)
	rec, err := scanRecord(row)
	if errors.Is(err, sql.ErrNoRows) {
		return FileRecord{}, errRecordNotFound
	}
	return rec, err
}

// findLiveRecordByName returns the (at most one, unless append-only) live
// record with the given display name.
func findLiveRecordByName(db execer, displayName string) (*FileRecord, error) {
	row := db.QueryRow(
		"SELECT "+recordColumns+" FROM files WHERE display_name = ? AND deleted = 0 AND pending = 0 LIMIT 1",
		displayName,
	)
	rec, err := scanRecord(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

// latestLiveRecordByName returns the most recently uploaded live record with
// the given display name - the one that /f/{filename} resolves to.
// Under ART_APPEND_ONLY, several live records can share a display name; this
// picks the newest one.
func latestLiveRecordByName(displayName string) (*FileRecord, error) {
	row := appDB.QueryRow(
		"SELECT "+recordColumns+" FROM files WHERE display_name = ? AND deleted = 0 AND pending = 0 ORDER BY modified DESC LIMIT 1",
		displayName,
	)
	rec, err := scanRecord(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

// orderByClause maps an "order" query param to a whitelisted ORDER BY
// fragment - never build this from raw user input, to avoid SQL injection.
func orderByClause(order string) string {
	switch order {
	case "name":
		return "display_name ASC"
	case "-name":
		return "display_name DESC"
	case "date":
		return "modified ASC"
	case "size":
		return "size ASC"
	case "-size":
		return "size DESC"
	default: // "-date" and anything unrecognized
		return "modified DESC"
	}
}

// escapeLikePattern escapes LIKE wildcard characters in a user-supplied
// search term so they're matched literally.
func escapeLikePattern(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "%", "\\%")
	s = strings.ReplaceAll(s, "_", "\\_")
	return s
}

// fileStats returns the live file count, summed size, and most recent
// modification time (empty if there are no live files) - computed by sqlite
// directly (COUNT/SUM/MAX) rather than in Go.
func fileStats() (totalFiles int, totalSize int64, lastUpload string, err error) {
	var lastUploadNull sql.NullString
	err = appDB.QueryRow(
		"SELECT COUNT(*), COALESCE(SUM(size), 0), MAX(modified) FROM files WHERE deleted = 0 AND pending = 0",
	).Scan(&totalFiles, &totalSize, &lastUploadNull)
	if err != nil {
		return 0, 0, "", err
	}
	return totalFiles, totalSize, lastUploadNull.String, nil
}

// listLiveRecords returns up to limit non-deleted, non-pending records
// starting at the offset-th live record matching search (a substring match
// against the display name), ordered per order - computed by sqlite directly
// (WHERE / ORDER BY / LIMIT / OFFSET) rather than in Go. The caller pages
// until it receives an empty result, so no total count is computed here.
func listLiveRecords(offset, limit int, search, order string) ([]FileRecord, error) {
	where := "WHERE deleted = 0 AND pending = 0"
	var args []any
	if search != "" {
		where += " AND display_name LIKE ? ESCAPE '\\'"
		args = append(args, "%"+escapeLikePattern(search)+"%")
	}

	rows, err := appDB.Query(
		"SELECT "+recordColumns+" FROM files "+where+" ORDER BY "+orderByClause(order)+" LIMIT ? OFFSET ?",
		append(append([]any{}, args...), limit, offset)...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []FileRecord
	for rows.Next() {
		rec, err := scanRecord(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, rec)
	}
	return records, rows.Err()
}

// reserveUpload creates a new pending record for displayName and, unless
// appendOnly is set, soft-deletes the previous live record with the same
// name (returned so the caller can reclaim its physical blob once the new
// upload succeeds). Soft-deleted rows are kept forever so their slug is
// never reused.
func reserveUpload(displayName, mimeType string) (rec FileRecord, replaced *FileRecord, err error) {
	tx, err := appDB.Begin()
	if err != nil {
		return FileRecord{}, nil, err
	}
	defer tx.Rollback()

	if !appendOnly {
		replaced, err = findLiveRecordByName(tx, displayName)
		if err != nil {
			return FileRecord{}, nil, err
		}
		if replaced != nil {
			if _, err := tx.Exec("UPDATE files SET deleted = 1 WHERE slug = ?", replaced.Slug); err != nil {
				return FileRecord{}, nil, err
			}
		}
	}

	rec, err = insertUniqueRecord(tx, FileRecord{DisplayName: displayName, MimeType: mimeType, Pending: true})
	if err != nil {
		return FileRecord{}, nil, err
	}

	if err := tx.Commit(); err != nil {
		return FileRecord{}, nil, err
	}
	return rec, replaced, nil
}

// finalizeUpload patches the record with its final size/modified time and
// clears Pending once the file has been fully written to disk.
func finalizeUpload(slug string, size int64, modified string) (FileRecord, error) {
	if _, err := appDB.Exec("UPDATE files SET size = ?, modified = ?, pending = 0 WHERE slug = ?", size, modified, slug); err != nil {
		return FileRecord{}, err
	}
	return getFileRecord(slug)
}

// abortUpload marks a reserved record as deleted after a failed write so its
// slug is never reused, without ever leaving it visible as a live file.
func abortUpload(slug string) error {
	_, err := appDB.Exec("UPDATE files SET deleted = 1, pending = 0 WHERE slug = ?", slug)
	return err
}

// softDeleteBySlug marks the live record for slug as deleted, keeping the row
// (and its slug) around forever.
func softDeleteBySlug(slug string) (*FileRecord, error) {
	tx, err := appDB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	row := tx.QueryRow(
		"SELECT "+recordColumns+" FROM files WHERE slug = ? AND deleted = 0 AND pending = 0",
		slug,
	)
	rec, err := scanRecord(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec("UPDATE files SET deleted = 1 WHERE slug = ?", rec.Slug); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &rec, nil
}

func shortLinkHandler(w http.ResponseWriter, r *http.Request) {
	slug := mux.Vars(r)["slug"]

	rec, err := getFileRecord(slug)
	if errors.Is(err, errRecordNotFound) {
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
