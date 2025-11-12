package main

import (
	"encoding/json"
	"net/http"
)

const (
	defaultMaxFileSize = 100 * 1024 * 1024 // 100MB
	chunkSize          = 8 * 1024 * 1024   // 8MB chunks
)

var (
	uploadFolder     string
	maxContentLength int64
	apiToken         string
	staticFolder     string
)

type ConfigResponse struct {
	Success          bool   `json:"success"`
	MaxContentLength int64  `json:"max_content_length"`
	Error            string `json:"error,omitempty"`
}

func getConfigHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ConfigResponse{
		Success:          true,
		MaxContentLength: maxContentLength,
	})
}
