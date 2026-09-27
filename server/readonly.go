package main

import (
	"encoding/json"
	"net/http"
)

// rejectWhenReadOnly refuses the routes that change data on an instance that
// only serves.
//
// Read-only is enforced here rather than by leaving the routes unregistered, so
// the API surface stays the same and a client gets a clear 403 instead of a 404
// that looks like a missing endpoint. It is also the last line of defence
// behind the read-only sqlite handle: even if a handler were reached, the
// database itself would refuse the write.
func rejectWhenReadOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if readOnly {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(Response{
				Success: false,
				Error:   "This instance is read-only",
			})
			return
		}
		next(w, r)
	}
}
