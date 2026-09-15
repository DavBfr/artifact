package main

import (
	"encoding/json"
	"net/http"
)

type StatsResponse struct {
	Success    bool   `json:"success"`
	TotalFiles int    `json:"total_files"`
	TotalSize  int64  `json:"total_size"`
	LastUpload string `json:"last_upload,omitempty"`
	Error      string `json:"error,omitempty"`
}

func statsRouteHandler(w http.ResponseWriter, r *http.Request) {
	if noListing {
		requireToken(statsHandler)(w, r)
		return
	}
	statsHandler(w, r)
}

func statsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	totalFiles, totalSize, lastUpload, err := fileStats()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(StatsResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(StatsResponse{
		Success:    true,
		TotalFiles: totalFiles,
		TotalSize:  totalSize,
		LastUpload: lastUpload,
	})
}
