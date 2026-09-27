package main

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Tags are docker-style "name:suffix" labels. A tag is globally unique and
// always resolves to exactly one file, but a single file can carry any number
// of tags. Names may be namespaced with slashes ("pets/cat:6.0"); only the
// suffix is restricted to a single path-free segment.
const (
	tagSeparator     = ":"
	defaultTagSuffix = "latest"
	maxTagLength     = 255
	// maxTagFieldSize caps a single multipart "tags" form field so a client
	// can't make the server buffer an arbitrary amount of text as a tag.
	maxTagFieldSize = 4096

	tagNameCharsetHint   = "letters, digits, '_', '-', '.' and '/' as a namespace separator"
	tagSuffixCharsetHint = "letters, digits, '_', '-' and '.'"
)

// tagsDDL creates the tag table. Deleting a file cascades to its tags; because
// this app soft-deletes (rows are kept forever so a slug is never reused), that
// cascade is a safety net on top of the explicit delete in softDeleteBySlug.
const tagsDDL = `
	CREATE TABLE IF NOT EXISTS tags (
		name    TEXT NOT NULL,
		suffix  TEXT NOT NULL,
		slug    TEXT NOT NULL REFERENCES files(slug) ON DELETE CASCADE,
		created TEXT NOT NULL DEFAULT '',
		PRIMARY KEY (name, suffix)
	);
	CREATE INDEX IF NOT EXISTS idx_tags_slug ON tags(slug);
`

// Tag is the parsed form of an API-facing "name:suffix" label. It deliberately
// carries no slug: where a tag points is whatever the tags table says it is,
// which is what lets adding a tag move it between files.
type Tag struct {
	Name   string
	Suffix string
}

// String renders the canonical API form of the tag.
func (t Tag) String() string { return t.Name + tagSeparator + t.Suffix }

// TagInfo is the API-facing view of a tag, resolved through the file it points
// at - used by the global tag listing.
type TagInfo struct {
	Name     string `json:"name"`
	Suffix   string `json:"suffix"`
	Tag      string `json:"tag"`
	Slug     string `json:"slug"`
	URL      string `json:"url"`
	FileName string `json:"file_name"`
	Created  string `json:"created"`
}

// parseTag parses the API-facing "name:suffix" tag format. A bare name means
// "<name>:latest", matching docker. The split happens on the LAST colon, so any
// earlier colon lands inside the name and is rejected by validation (names
// cannot contain colons).
func parseTag(raw string) (Tag, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return Tag{}, errors.New("tag cannot be empty")
	}

	name, suffix := value, defaultTagSuffix
	if idx := strings.LastIndex(value, tagSeparator); idx >= 0 {
		name, suffix = value[:idx], value[idx+1:]
	}

	if !isValidTagName(name) {
		return Tag{}, fmt.Errorf("invalid tag name %q: use %s", name, tagNameCharsetHint)
	}
	if !isValidTagSuffix(suffix) {
		return Tag{}, fmt.Errorf("invalid tag suffix %q: use %s", suffix, tagSuffixCharsetHint)
	}
	if len(name)+len(suffix)+1 > maxTagLength {
		return Tag{}, fmt.Errorf("tag %q is too long (max %d characters)", value, maxTagLength)
	}

	return Tag{Name: name, Suffix: suffix}, nil
}

// parseTagList parses and de-duplicates the tags from a request, keeping the
// caller's order. An empty input yields an empty slice (not an error) - callers
// that require at least one tag check for that themselves.
func parseTagList(raw []string) ([]Tag, error) {
	tags := make([]Tag, 0, len(raw))
	seen := make(map[string]bool, len(raw))
	for _, item := range raw {
		tag, err := parseTag(item)
		if err != nil {
			return nil, err
		}
		key := tag.String()
		if seen[key] {
			continue
		}
		seen[key] = true
		tags = append(tags, tag)
	}
	return tags, nil
}

// isValidTagName reports whether name is a usable (possibly namespaced) tag name.
func isValidTagName(name string) bool {
	if name == "" || strings.HasPrefix(name, "/") || strings.HasSuffix(name, "/") {
		return false
	}
	// Reject empty, "." and ".." segments so a tag always reads as a clean path.
	for _, segment := range strings.Split(name, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
	}
	return isTagChars(name, true)
}

// isValidTagSuffix reports whether suffix is a usable tag suffix (no slashes).
func isValidTagSuffix(suffix string) bool {
	return suffix != "" && isTagChars(suffix, false)
}

// isTagChars reports whether value only contains tag-safe ASCII characters.
// Tags are case-sensitive, so no case folding happens here.
func isTagChars(value string, allowSlash bool) bool {
	for i := 0; i < len(value); i++ {
		c := value[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case c == '_' || c == '-' || c == '.':
		case c == '/' && allowSlash:
		default:
			return false
		}
	}
	return true
}

// liveSlugExists reports whether slug is a live record. Tags may only be
// attached to live files, never to deleted or still-pending ones.
func liveSlugExists(db execer, slug string) (bool, error) {
	var exists int
	err := db.QueryRow(
		"SELECT 1 FROM files WHERE slug = ? AND deleted = 0 AND pending = 0",
		slug,
	).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// upsertTag points tag at slug, moving it off whatever file it currently
// describes: a tag always resolves to exactly one file, so re-adding an
// existing tag re-points it rather than failing.
func upsertTag(db execer, tag Tag, slug, now string) error {
	_, err := db.Exec(
		`INSERT INTO tags (name, suffix, slug, created) VALUES (?, ?, ?, ?)
		 ON CONFLICT(name, suffix) DO UPDATE SET slug = excluded.slug, created = excluded.created`,
		tag.Name, tag.Suffix, slug, now,
	)
	return err
}

// tagOwner returns the slug a tag currently points at, or "" if the tag is unused.
func tagOwner(db execer, tag Tag) (string, error) {
	var slug string
	err := db.QueryRow(
		"SELECT slug FROM tags WHERE name = ? AND suffix = ?",
		tag.Name, tag.Suffix,
	).Scan(&slug)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return slug, nil
}

// tagsOfSlug returns the canonical tags attached to slug, ordered
// deterministically. The result is never nil.
func tagsOfSlug(db execer, slug string) ([]string, error) {
	rows, err := db.Query(
		"SELECT name, suffix FROM tags WHERE slug = ? ORDER BY name ASC, suffix ASC",
		slug,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tags := []string{}
	for rows.Next() {
		var name, suffix string
		if err := rows.Scan(&name, &suffix); err != nil {
			return nil, err
		}
		tags = append(tags, name+tagSeparator+suffix)
	}
	return tags, rows.Err()
}

// tagsForSlugs resolves the tags of a whole page of slugs in one query, so
// decorating /api/files never becomes an N+1 query pattern.
func tagsForSlugs(slugs []string) (map[string][]string, error) {
	tagsBySlug := make(map[string][]string, len(slugs))
	if len(slugs) == 0 {
		return tagsBySlug, nil
	}

	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(slugs)), ",")
	args := make([]any, 0, len(slugs))
	for _, slug := range slugs {
		args = append(args, slug)
	}

	rows, err := db().Query(
		"SELECT slug, name, suffix FROM tags WHERE slug IN ("+placeholders+") ORDER BY name ASC, suffix ASC",
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var slug, name, suffix string
		if err := rows.Scan(&slug, &name, &suffix); err != nil {
			return nil, err
		}
		tagsBySlug[slug] = append(tagsBySlug[slug], name+tagSeparator+suffix)
	}
	return tagsBySlug, rows.Err()
}

// tagsOwnedByAnotherFile returns the slugs, other than exclude, that currently
// own any of tags. An empty result means none of them would be taken off another
// file, so the request does not need the permission to remove one.
func tagsOwnedByAnotherFile(tags []Tag, exclude string) ([]string, error) {
	var owners []string
	for _, tag := range tags {
		owner, err := tagOwner(db(), tag)
		if err != nil {
			return nil, err
		}
		if owner != "" && owner != exclude {
			owners = append(owners, owner)
		}
	}
	return owners, nil
}

// addTagsToSlug attaches tags to a live slug, moving each tag off any other
// file. It returns the slug's full tag list after the change, plus the tags
// that were re-pointed away from a different file (tag -> previous slug).
func addTagsToSlug(slug string, tags []Tag) ([]string, map[string]string, error) {
	tx, err := db().Begin()
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback()

	now := time.Now().UTC().Format(time.RFC3339)
	moved := map[string]string{}
	for _, tag := range tags {
		previous, err := tagOwner(tx, tag)
		if err != nil {
			return nil, nil, err
		}
		if previous != "" && previous != slug {
			moved[tag.String()] = previous
		}
		if err := upsertTag(tx, tag, slug, now); err != nil {
			return nil, nil, err
		}
	}

	list, err := tagsOfSlug(tx, slug)
	if err != nil {
		return nil, nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, nil, err
	}
	return list, moved, nil
}

// removeTagFromSlug detaches one tag from slug, reporting whether it was
// actually attached to that slug.
func removeTagFromSlug(slug string, tag Tag) (bool, error) {
	res, err := db().Exec(
		"DELETE FROM tags WHERE name = ? AND suffix = ? AND slug = ?",
		tag.Name, tag.Suffix, slug,
	)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

// deleteTagsForSlug drops every tag pointing at slug. Unlike slugs (which are
// never reused), tag names are deliberately freed for later use.
func deleteTagsForSlug(db execer, slug string) error {
	_, err := db.Exec("DELETE FROM tags WHERE slug = ?", slug)
	return err
}

// applyUploadTags finishes an upload: every tag named in the request is
// re-pointed to newSlug, and the replaced file is retired - its tags are dropped
// (except any re-listed just above) and its record is soft-deleted. Both happen
// in one transaction, so a superseded file is never left live with its tags
// moved away, and it is retired only now that the upload has succeeded.
//
// Order matters: the requested tags must move off replacedSlug before the sweep,
// otherwise one would be deleted right after being moved.
func applyUploadTags(newSlug string, tags []Tag, replacedSlug string) error {
	if len(tags) == 0 && replacedSlug == "" {
		return nil
	}

	tx, err := db().Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now().UTC().Format(time.RFC3339)
	for _, tag := range tags {
		if err := upsertTag(tx, tag, newSlug, now); err != nil {
			return err
		}
	}

	if replacedSlug != "" {
		if err := deleteTagsForSlug(tx, replacedSlug); err != nil {
			return err
		}
		if _, err := tx.Exec("UPDATE files SET deleted = 1 WHERE slug = ?", replacedSlug); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// liveRecordByTag resolves a tag to the live file it points at, or nil if the
// tag is unknown or its file has been deleted.
func liveRecordByTag(name, suffix string) (*FileRecord, error) {
	row := db().QueryRow(
		"SELECT "+recordColumns+" FROM files WHERE slug = (SELECT slug FROM tags WHERE name = ? AND suffix = ?)"+
			" AND deleted = 0 AND pending = 0",
		name, suffix,
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

// listAllTags returns the tags of live files, ordered by name then suffix, page
// by page - computed by sqlite directly (JOIN / WHERE / ORDER BY / LIMIT /
// OFFSET). The result is never nil.
func listAllTags(offset, limit int, search string) ([]TagInfo, error) {
	where := "WHERE f.deleted = 0 AND f.pending = 0"
	var args []any
	if search != "" {
		where += " AND (t.name LIKE ? ESCAPE '\\' OR t.suffix LIKE ? ESCAPE '\\')"
		pattern := "%" + escapeLikePattern(search) + "%"
		args = append(args, pattern, pattern)
	}

	rows, err := db().Query(
		`SELECT t.name, t.suffix, t.slug, t.created, f.display_name
		 FROM tags t JOIN files f ON f.slug = t.slug `+where+
			` ORDER BY t.name ASC, t.suffix ASC LIMIT ? OFFSET ?`,
		append(append([]any{}, args...), limit, offset)...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tags := make([]TagInfo, 0, limit)
	for rows.Next() {
		var info TagInfo
		if err := rows.Scan(&info.Name, &info.Suffix, &info.Slug, &info.Created, &info.FileName); err != nil {
			return nil, err
		}
		info.Tag = info.Name + tagSeparator + info.Suffix
		info.URL = "/t/" + info.Tag
		tags = append(tags, info)
	}
	return tags, rows.Err()
}
