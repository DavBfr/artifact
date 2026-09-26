package main

import (
	"net/http"

	"github.com/gorilla/mux"
)

// tagDownloadHandler serves /t/{tag}: the file a tag currently points at.
// The route uses a {tag:.+} variable so tags whose name is namespaced
// ("pets/cat:6.0") arrive here as one variable, mirroring /s/{slug}.
func tagDownloadHandler(w http.ResponseWriter, r *http.Request) {
	tag, err := parseTag(mux.Vars(r)["tag"])
	if err != nil {
		// A malformed tag can never have been stored, so it simply doesn't exist.
		http.NotFound(w, r)
		return
	}

	rec, err := liveRecordByTag(tag.Name, tag.Suffix)
	if err != nil {
		http.Error(w, "Failed to resolve tag", http.StatusInternalServerError)
		return
	}
	if rec == nil {
		http.NotFound(w, r)
		return
	}

	serveFileRecord(w, r, *rec)
}
