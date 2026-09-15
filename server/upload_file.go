package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
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

	var part *multipart.Part
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
		p.Close()
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

	destPath := filepath.Join(uploadFolder, rec.StorageKey)
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		_ = abortUpload(rec.Slug)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(UploadResponse{
			Success: false,
			Error:   "Failed to create storage directory: " + err.Error(),
		})
		return
	}

	// Create destination file
	dst, err := os.Create(destPath)
	if err != nil {
		_ = abortUpload(rec.Slug)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(UploadResponse{
			Success: false,
			Error:   "Failed to create file: " + err.Error(),
		})
		return
	}
	defer dst.Close()

	// Copy file with chunked reading for efficient memory usage, enforcing the
	// max size limit since the part's size isn't known ahead of time.
	buffer := make([]byte, chunkSize)
	limitedReader := io.LimitReader(part, maxContentLength+1)
	written, err := io.CopyBuffer(dst, limitedReader, buffer)
	if err != nil {
		dst.Close()
		os.Remove(destPath)
		_ = abortUpload(rec.Slug)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(UploadResponse{
			Success: false,
			Error:   "Failed to save file: " + err.Error(),
		})
		return
	}
	if written > maxContentLength {
		dst.Close()
		os.Remove(destPath)
		_ = abortUpload(rec.Slug)
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		json.NewEncoder(w).Encode(UploadResponse{
			Success: false,
			Error:   fmt.Sprintf("File too large. Maximum size is %d bytes", maxContentLength),
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

	// Now that the new upload is live, reclaim the replaced file's blob (best-effort).
	if replacedRec != nil {
		if err := os.Remove(filepath.Join(uploadFolder, replacedRec.StorageKey)); err != nil && !os.IsNotExist(err) {
			log.Printf("Failed to remove replaced file %s: %v", replacedRec.StorageKey, err)
		}
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(UploadResponse{
		Success:  true,
		Message:  "File uploaded successfully",
		File:     fileInfoFromRecord(finalRec),
		Replaced: replacedRec != nil,
	})
}
