package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

type UploadResponse struct {
	Success  bool     `json:"success"`
	Message  string   `json:"message,omitempty"`
	File     FileInfo `json:"file,omitempty"`
	Replaced bool     `json:"replaced"`
	Error    string   `json:"error,omitempty"`
}

func uploadFileHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Use the raw multipart reader instead of ParseMultipartForm/FormFile: the
	// latter spills large parts to disk via os.TempDir() ("/tmp"), which does
	// not exist in our scratch-based container image. Streaming the part
	// directly to the destination file avoids relying on a temp directory.
	reader, err := r.MultipartReader()
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(UploadResponse{
			Success: false,
			Error:   "Failed to parse form: " + err.Error(),
		})
		return
	}

	// Tags may be sent before or after the file part, so they're collected on
	// both passes: here while scanning for the file, and again once the file has
	// been streamed (the reader can only reach later parts after earlier ones).
	var part *multipart.Part
	var rawTags []string
	for {
		p, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(UploadResponse{
				Success: false,
				Error:   "Failed to parse form: " + err.Error(),
			})
			return
		}
		if p.FormName() == "file" {
			part = p
			break
		}
		value, isTag, tagErr := readTagField(p)
		p.Close()
		if tagErr != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(UploadResponse{
				Success: false,
				Error:   tagErr.Error(),
			})
			return
		}
		if isTag {
			rawTags = append(rawTags, value)
		}
	}

	if part == nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(UploadResponse{
			Success: false,
			Error:   "No file provided",
		})
		return
	}
	defer part.Close()

	if part.FileName() == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(UploadResponse{
			Success: false,
			Error:   "No file selected",
		})
		return
	}

	// Secure the display filename (basic version) - this is the name the file
	// is known by, independent of where its bytes live on disk.
	displayName := filepath.Base(filepath.Clean(part.FileName()))

	var mimeType string
	if ext := strings.ToLower(filepath.Ext(displayName)); ext != "" {
		mimeType = mime.TypeByExtension(ext)
	}

	// Reserve a db record (and slug) up front: unless append-only, this also
	// soft-deletes any existing live record for the same display name.
	rec, replacedRec, err := reserveUpload(displayName, mimeType)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(UploadResponse{
			Success: false,
			Error:   "Failed to reserve upload: " + err.Error(),
		})
		return
	}

	// Abandoning a reserved upload drops both halves of it: the record is marked
	// deleted (so its slug is never reused) and any blob already written is
	// removed. The cleanup runs detached from the request so a client that
	// walked away mid-upload can't leave the blob behind.
	discard := func() {
		_ = storage.Delete(context.WithoutCancel(r.Context()), rec.StorageKey)
		_ = abortUpload(rec.Slug)
	}

	// Stream the part straight into the blob store. The store enforces the size
	// limit while reading, since the part's size isn't known ahead of time.
	written, err := storage.Put(r.Context(), rec.StorageKey, part, maxContentLength)
	if err != nil {
		discard()
		if errors.Is(err, errBlobTooLarge) {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			json.NewEncoder(w).Encode(UploadResponse{
				Success: false,
				Error:   fmt.Sprintf("File too large. Maximum size is %d bytes", maxContentLength),
			})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(UploadResponse{
			Success: false,
			Error:   "Failed to save file: " + err.Error(),
		})
		return
	}

	// The file part has been consumed, so any parts after it are reachable now.
	for {
		p, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			discard()
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(UploadResponse{
				Success: false,
				Error:   "Failed to parse form: " + err.Error(),
			})
			return
		}
		value, isTag, tagErr := readTagField(p)
		p.Close()
		if tagErr != nil {
			discard()
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(UploadResponse{
				Success: false,
				Error:   tagErr.Error(),
			})
			return
		}
		if isTag {
			rawTags = append(rawTags, value)
		}
	}

	// Validate the tags before the upload goes live: a bad tag must not leave a
	// half-configured file behind.
	tags, err := parseTagList(rawTags)
	if err != nil {
		discard()
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(UploadResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	finalRec, err := finalizeUpload(rec.Slug, written, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(UploadResponse{
			Success: false,
			Error:   "Failed to finalize upload: " + err.Error(),
		})
		return
	}

	// Point the requested tags at the new upload, then drop whatever tags the
	// file it superseded still had (except any re-listed just above).
	replacedSlug := ""
	if replacedRec != nil {
		replacedSlug = replacedRec.Slug
	}
	if err := applyUploadTags(finalRec.Slug, tags, replacedSlug); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(UploadResponse{
			Success: false,
			Error:   "Failed to apply tags: " + err.Error(),
		})
		return
	}

	applied, err := tagsOfSlug(appDB, finalRec.Slug)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(UploadResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	// Now that the new upload is live, reclaim the replaced file's blob (best-effort).
	if replacedRec != nil {
		if err := storage.Delete(context.WithoutCancel(r.Context()), replacedRec.StorageKey); err != nil {
			log.Printf("Failed to remove replaced file %s: %v", replacedRec.StorageKey, err)
		}
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(UploadResponse{
		Success:  true,
		Message:  "File uploaded successfully",
		File:     fileInfoFromRecord(finalRec, applied),
		Replaced: replacedRec != nil,
	})
}

// readTagField reads a "tags" form field, reporting whether p was a tags field
// at all. The value is size-capped and trimmed, and an empty value is left for
// parseTag to reject rather than being silently dropped.
func readTagField(p *multipart.Part) (string, bool, error) {
	if p.FormName() != "tags" {
		return "", false, nil
	}
	value, err := io.ReadAll(io.LimitReader(p, maxTagFieldSize+1))
	if err != nil {
		return "", true, fmt.Errorf("Failed to read tags field: %v", err)
	}
	if len(value) > maxTagFieldSize {
		return "", true, fmt.Errorf("Tag is too long (max %d bytes)", maxTagFieldSize)
	}
	return strings.TrimSpace(string(value)), true, nil
}
