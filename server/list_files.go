package main

import (
	"encoding/json"
	"net/http"
	"sort"
)

type FileInfo struct {
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	Modified string `json:"modified"`
	URL      string `json:"url"`
	MimeType string `json:"mime_type"`
}

type ListFilesResponse struct {
	Success bool       `json:"success"`
	Files   []FileInfo `json:"files"`
	Count   int        `json:"count"`
	Error   string     `json:"error,omitempty"`
}

func listFilesRouteHandler(w http.ResponseWriter, r *http.Request) {
	if noListing {
		requireToken(listFilesHandler)(w, r)
		return
	}
	listFilesHandler(w, r)
}

func listFilesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// The db is the single source of truth for listing - no directory scan.
	records, err := listLiveRecords()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ListFilesResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	files := make([]FileInfo, 0, len(records))
	for _, rec := range records {
		files = append(files, fileInfoFromRecord(rec))
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
