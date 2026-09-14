package main

import (
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
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

	// Secure the filename (basic version)
	filename := filepath.Base(filepath.Clean(part.FileName()))
	destPath := filepath.Join(uploadFolder, filename)

	// Check if file already exists
	replaced := false
	if _, err := os.Stat(destPath); err == nil {
		if appendOnly {
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(UploadResponse{
				Success: false,
				Error:   "File already exists. Overwriting is disabled (append-only mode).",
			})
			return
		}
		replaced = true
	}

	// Create destination file
	dst, err := os.Create(destPath)
	if err != nil {
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
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		json.NewEncoder(w).Encode(UploadResponse{
			Success: false,
			Error:   fmt.Sprintf("File too large. Maximum size is %d bytes", maxContentLength),
		})
		return
	}

	// Get file info for response
	fileInfo, err := getFileInfo(destPath)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(UploadResponse{
			Success: false,
			Error:   "Failed to get file info: " + err.Error(),
		})
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(UploadResponse{
		Success:  true,
		Message:  "File uploaded successfully",
		File:     fileInfo,
		Replaced: replaced,
	})
}
