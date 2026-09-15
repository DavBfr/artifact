package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gorilla/mux"
)

type DeleteResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

func deleteFileHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if appendOnly {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(DeleteResponse{
			Success: false,
			Error:   "Deletion is disabled (append-only mode).",
		})
		return
	}

	vars := mux.Vars(r)
	filename := filepath.Base(filepath.Clean(vars["filename"]))

	// Soft-delete: the row (and its slug) is kept forever so it can never be reused.
	rec, err := softDeleteByDisplayName(filename)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(DeleteResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}
	if rec == nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(DeleteResponse{
			Success: false,
			Error:   "File not found",
		})
		return
	}

	// The record is already marked deleted and will never be served again,
	// so reclaiming the physical blob is best-effort.
	if err := os.Remove(filepath.Join(uploadFolder, rec.StorageKey)); err != nil && !os.IsNotExist(err) {
		log.Printf("Failed to remove deleted file %s: %v", rec.StorageKey, err)
	}

	json.NewEncoder(w).Encode(DeleteResponse{
		Success: true,
		Message: "File deleted successfully",
	})
}
