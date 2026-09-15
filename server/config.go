package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
)

const (
	defaultMaxFileSize  = 100 * 1024 * 1024 // 100MB
	chunkSize           = 8 * 1024 * 1024   // 8MB chunks
	defaultMaxListLimit = 500
	dbFileName          = "artifact.db"
)

var (
	uploadFolder     string
	maxContentLength int64
	apiToken         string
	staticFolder     string
	webPortal        bool
	noListing        bool
	appendOnly       bool
	noFilenameURL    bool
	appDB            *sql.DB
	maxListLimit     int
)

type ConfigResponse struct {
	Success             bool   `json:"success"`
	MaxContentLength    int64  `json:"max_content_length,omitempty"`
	MaxListLimit        int    `json:"max_list_limit,omitempty"`
	FilenameURLsEnabled bool   `json:"filename_urls_enabled,omitempty"`
	Error               string `json:"error,omitempty"`
}

// getConfigHandler is public, but only reveals max_content_length (and
// max_list_limit when listing is private) to callers with a valid token.
// filename_urls_enabled is never sensitive, so it's public too, but (via
// omitempty) only present in the response when actually true.
func getConfigHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if authHeader := r.Header.Get("Authorization"); authHeader != "" {
		if apiToken == "" || strings.TrimPrefix(authHeader, "Bearer ") != apiToken {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(ConfigResponse{
				Success: false,
				Error:   "Invalid authentication token",
			})
			return
		}

		json.NewEncoder(w).Encode(ConfigResponse{
			Success:             true,
			MaxContentLength:    maxContentLength,
			MaxListLimit:        maxListLimit,
			FilenameURLsEnabled: !noFilenameURL,
		})
		return
	}

	// No token supplied: public, reduced response.
	resp := ConfigResponse{
		Success: true,
	}
	if !noFilenameURL {
		resp.FilenameURLsEnabled = true
	}
	if !noListing {
		resp.MaxListLimit = maxListLimit
	}
	json.NewEncoder(w).Encode(resp)
}
