package main

import (
	"context"
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

// writeForbidden sends the JSON 403 used when a credential is valid but not
// allowed to perform this particular change. The message names the missing
// permission, because "forbidden" alone leaves a caller nothing to act on.
func writeForbidden(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	json.NewEncoder(w).Encode(Response{
		Success: false,
		Error:   message,
	})
}

// permissionDeniedError is the message for a credential that lacks perm.
func permissionDeniedError(perm string) string {
	return "Access denied: this credential does not have the " + perm + " permission"
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

// authKind says which kind of credential authorized a request, so handlers and
// logs can tell the shared static token apart from a per-user session.
const (
	authKindStatic  = "static"
	authKindSession = "session"
)

// requestAuth is the resolved credential behind a request: who it is, and what
// it may do. It is attached to the request context by requireToken, so a handler
// can check a payload-dependent permission without re-verifying the token.
type requestAuth struct {
	Kind    string
	Subject string
	Via     string
	Perms   permissionSet
}

// requestAuthKey is the context key for the resolved credential. An unexported
// struct type keeps it from colliding with anything else stashed on a request.
type requestAuthKey struct{}

// requestAuthFrom returns the credential resolved for r, or nil when the request
// did not come through requireToken (a public route).
func requestAuthFrom(r *http.Request) *requestAuth {
	auth, _ := r.Context().Value(requestAuthKey{}).(*requestAuth)
	return auth
}

// resolveAuth returns the credential a request carries, and whether it is valid.
// The static token is compared first because it needs no parsing and no signature
// check. It is also the one credential that always carries every permission: it
// predates roles, and the CI pipelines and curl examples rely on it working
// exactly as before.
func resolveAuth(r *http.Request) (*requestAuth, bool) {
	presented := bearerToken(r)
	if presented == "" {
		return nil, false
	}

	if apiToken != "" && subtle.ConstantTimeCompare([]byte(presented), []byte(apiToken)) == 1 {
		return &requestAuth{Kind: authKindStatic, Perms: permissionSetFrom(allPermissions)}, true
	}

	if claims, err := verifySessionToken(presented); err == nil {
		return &requestAuth{
			Kind:    authKindSession,
			Subject: claims.Subject,
			Via:     claims.Via,
			Perms:   permissionSetFrom(claims.Perms),
		}, true
	}

	return nil, false
}

// authenticateRequest reports whether a request carries a valid credential.
func authenticateRequest(r *http.Request) bool {
	_, ok := resolveAuth(r)
	return ok
}

// hasPermission reports whether the request's credential holds perm, without
// writing anything - useful when a handler has cleanup to do before it answers.
func hasPermission(r *http.Request, perm string) bool {
	auth := requestAuthFrom(r)
	return auth != nil && auth.Perms.has(perm)
}

// firstMissingPermission returns the first permission in perms the request's
// credential does not hold, or "" when it holds them all. Taking a list keeps a
// handler from having to write its response between two failed checks.
func firstMissingPermission(r *http.Request, perms ...string) string {
	for _, perm := range perms {
		if !hasPermission(r, perm) {
			return perm
		}
	}
	return ""
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

		auth, ok := resolveAuth(r)
		if !ok {
			writeUnauthorized(w, "Invalid authentication token")
			return
		}

		next(w, r.WithContext(context.WithValue(r.Context(), requestAuthKey{}, auth)))
	}
}

// requirePermission refuses a request whose credential lacks perm. It wraps a
// handler inside requireToken, so an unauthenticated caller still gets exactly
// the 401 it always got and learns nothing about what the route requires.
func requirePermission(perm string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !inHandlerPermission(w, r, perm) {
			return
		}
		next(w, r)
	}
}

// inHandlerPermission checks a permission from inside a handler. Two routes need
// this: uploading and adding tags both have effects that depend on the request
// payload - a same-named upload supersedes a file, a tag that already points
// elsewhere is moved off it - so neither can carry one fixed permission.
//
// It writes the 403 itself and reports whether the caller may proceed, so a
// handler must do any cleanup before calling it.
func inHandlerPermission(w http.ResponseWriter, r *http.Request, perm string) bool {
	if hasPermission(r, perm) {
		return true
	}
	writeForbidden(w, permissionDeniedError(perm))
	return false
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
