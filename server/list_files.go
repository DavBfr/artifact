package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
)



type FileInfo struct {
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	Modified string `json:"modified"`
	URL      string `json:"url"`
}


type ListFilesResponse struct {
	Success bool       `json:"success"`
	Files   []FileInfo `json:"files"`
	Count   int        `json:"count"`
	Error   string     `json:"error,omitempty"`
}


func listFilesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	files := []FileInfo{}

	entries, err := os.ReadDir(uploadFolder)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ListFilesResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			filePath := filepath.Join(uploadFolder, entry.Name())
			fileInfo, err := getFileInfo(filePath)
			if err == nil {
				files = append(files, fileInfo)
			}
		}
	}

	// Sort by modification time (newest first)
	sort.Slice(files, func(i, j int) bool {
		return files[i].Modified > files[j].Modified
	})

	json.NewEncoder(w).Encode(ListFilesResponse{
		Success: true,
		Files:   files,
		Count:   len(files),
	})
}
