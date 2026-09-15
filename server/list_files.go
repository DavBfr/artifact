package main

import (
	"encoding/json"
	"net/http"
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

	// limit is capped by maxListLimit (ART_MAX_LIST_LIMIT) regardless of what the client asks for
	limit := parseInt(r.URL.Query().Get("limit"), maxListLimit)
	if limit <= 0 || limit > maxListLimit {
		limit = maxListLimit
	}
	offset := parseInt(r.URL.Query().Get("offset"), 0)
	if offset < 0 {
		offset = 0
	}

	search := r.URL.Query().Get("search")
	order := r.URL.Query().Get("order")

	// The db is the single source of truth for listing - no directory scan.
	// The UI paginates until it receives an empty page, so no total count is returned.
	records, err := listLiveRecords(offset, limit, search, order)
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

	json.NewEncoder(w).Encode(ListFilesResponse{
		Success: true,
		Files:   files,
		Count:   len(files),
	})
}
