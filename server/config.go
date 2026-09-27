package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
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
	OidcEnabled         bool   `json:"oidc_enabled,omitempty"`
	Error               string `json:"error,omitempty"`
}

// getConfigHandler is public, but only reveals max_content_length (and
// max_list_limit when listing is private) to callers with a valid credential.
// filename_urls_enabled is never sensitive, so it's public too, but (via
// omitempty) only present in the response when actually true. oidc_enabled is
// public because the UI must know to send the browser to the provider instead
// of asking for a token.
func getConfigHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	resp := ConfigResponse{
		Success:     true,
		OidcEnabled: oidcEnabled(),
	}
	if !noFilenameURL {
		resp.FilenameURLsEnabled = true
	}
	if !noListing {
		resp.MaxListLimit = maxListLimit
	}

	// A supplied credential is either good - in which case the caller also sees
	// the values that only make sense with one - or it gets the 401 a wrong
	// token has always produced.
	if r.Header.Get("Authorization") != "" {
		if !authenticateRequest(r) {
			writeUnauthorized(w, "Invalid authentication token")
			return
		}
		resp.MaxContentLength = maxContentLength
		resp.MaxListLimit = maxListLimit
		resp.FilenameURLsEnabled = !noFilenameURL
	}

	json.NewEncoder(w).Encode(resp)
}
