package main

import (
	"crypto/subtle"
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

// writeUnauthorized sends the JSON 401 every auth failure uses.
func writeUnauthorized(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(Response{
		Success: false,
		Error:   message,
	})
}

// authConfigured reports whether the server has any way to authenticate a
// request at all. ART_SESSION_SECRET counts on its own: being able to verify
// (and mint) session tokens is enough to enforce auth on uploads and deletes,
// rather than refusing every request as unconfigurable.
func authConfigured() bool {
	return apiToken != "" || sessionAuthEnabled()
}

// bearerToken returns the credential from the Authorization header, tolerating
// a missing "Bearer " prefix - the header shape predates this server and the
// curl examples in the README rely on it.
func bearerToken(r *http.Request) string {
	return strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
}

// authenticateRequest reports whether a request carries a valid credential. The
// static token is compared first because it needs no parsing and no signature
// check; a session token is only considered when ART_SESSION_SECRET is set.
func authenticateRequest(r *http.Request) bool {
	presented := bearerToken(r)
	if presented == "" {
		return false
	}

	if apiToken != "" && subtle.ConstantTimeCompare([]byte(presented), []byte(apiToken)) == 1 {
		return true
	}

	if _, err := verifySessionToken(presented); err == nil {
		return true
	}

	return false
}

func requireToken(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		if !authConfigured() {
			// Neither ART_API_TOKEN nor ART_SESSION_SECRET is set, so no
			// credential could ever be valid.
			writeUnauthorized(w, "Access denied. No API token configured on server.")
			return
		}

		if bearerToken(r) == "" {
			writeUnauthorized(w, "Authentication required. Provide a token in the Authorization header.")
			return
		}

		if !authenticateRequest(r) {
			writeUnauthorized(w, "Invalid authentication token")
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
