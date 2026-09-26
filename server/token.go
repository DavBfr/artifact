package main

import (
	"encoding/json"
	"net/http"
	"strings"
)

type Response struct {
	Success bool        `json:"success"`
	Error   string      `json:"error,omitempty"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

func requireToken(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		if apiToken == "" {
			// Authenticated access disabled
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(Response{
				Success: false,
				Error:   "Access denied. No API token configured on server.",
			})
			return
		}

		token := r.Header.Get("Authorization")

		if token == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(Response{
				Success: false,
				Error:   "Authentication required. Provide token in Authorization header or token form field.",
			})
			return
		}

		// Remove 'Bearer ' prefix if present
		token = strings.TrimPrefix(token, "Bearer ")

		if token != apiToken {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(Response{
				Success: false,
				Error:   "Invalid authentication token",
			})
			return
		}

		next(w, r)
	}
}

// mayRequireToken leaves a handler public by default but requires a valid API
// token when private listing is enabled (ART_NO_LISTING). Only the two listing
// routes (GET /api/files, GET /api/stats) behave this way.
//
// noListing is checked per request rather than when the wrapper is applied, so
// behaviour doesn't depend on whether the route is registered before or after
// the configuration is read.
func mayRequireToken(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if noListing {
			requireToken(next)(w, r)
			return
		}
		next(w, r)
	}
}
