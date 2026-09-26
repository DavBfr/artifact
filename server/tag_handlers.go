package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
)

type TagListResponse struct {
	Success bool      `json:"success"`
	Tags    []TagInfo `json:"tags"`
	Count   int       `json:"count"`
	Error   string    `json:"error,omitempty"`
}

type FileTagsResponse struct {
	Success bool     `json:"success"`
	Slug    string   `json:"slug"`
	Tags    []string `json:"tags"`
	// Moved maps each tag that was re-pointed away from another file to that
	// file's previous slug, so a caller can see what it took the tag from.
	Moved   map[string]string `json:"moved,omitempty"`
	Message string            `json:"message,omitempty"`
	Error   string            `json:"error,omitempty"`
}

type AddTagsRequest struct {
	Tags []string `json:"tags"`
}

// listAllTagsHandler serves GET /api/tags: every tag attached to a live file,
// paged. Wrapped in mayRequireToken when registered.
func listAllTagsHandler(w http.ResponseWriter, r *http.Request) {
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

	tags, err := listAllTags(offset, limit, r.URL.Query().Get("search"))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(TagListResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(TagListResponse{
		Success: true,
		Tags:    tags,
		Count:   len(tags),
	})
}

// listFileTagsHandler serves GET /api/tags/{slug}. Wrapped in mayRequireToken
// when registered.
func listFileTagsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	slug := mux.Vars(r)["slug"]

	// Only live files can carry tags, so a deleted or still-pending slug is a 404.
	live, err := liveSlugExists(appDB, slug)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(FileTagsResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}
	if !live {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(FileTagsResponse{
			Success: false,
			Error:   "File not found",
		})
		return
	}

	tags, err := tagsOfSlug(appDB, slug)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(FileTagsResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(FileTagsResponse{
		Success: true,
		Slug:    slug,
		Tags:    tags,
	})
}

// addFileTagsHandler serves POST /api/tags/{slug}, attaching tags to a file.
// Because a tag always points at exactly one file, adding a tag that currently
// belongs to another file moves it.
func addFileTagsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	slug := mux.Vars(r)["slug"]

	var req AddTagsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(FileTagsResponse{
			Success: false,
			Error:   "Invalid JSON body: " + err.Error(),
		})
		return
	}

	tags, err := parseTagList(req.Tags)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(FileTagsResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}
	if len(tags) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(FileTagsResponse{
			Success: false,
			Error:   "No tags provided",
		})
		return
	}

	live, err := liveSlugExists(appDB, slug)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(FileTagsResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}
	if !live {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(FileTagsResponse{
			Success: false,
			Error:   "File not found",
		})
		return
	}

	list, moved, err := addTagsToSlug(slug, tags)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(FileTagsResponse{
			Success: false,
			Error:   "Failed to add tags: " + err.Error(),
		})
		return
	}

	resp := FileTagsResponse{
		Success: true,
		Slug:    slug,
		Tags:    list,
	}
	if len(moved) > 0 {
		resp.Moved = moved
		resp.Message = "One or more tags were moved from another file"
	}
	json.NewEncoder(w).Encode(resp)
}

// removeFileTagHandler serves DELETE /api/tags/{slug}/{tag}. The tag may contain
// slashes, so it is registered with a {tag:.+} variable.
func removeFileTagHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	slug := mux.Vars(r)["slug"]

	tag, err := parseTag(mux.Vars(r)["tag"])
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(FileTagsResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	removed, err := removeTagFromSlug(slug, tag)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(FileTagsResponse{
			Success: false,
			Error:   "Failed to remove tag: " + err.Error(),
		})
		return
	}
	if !removed {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(FileTagsResponse{
			Success: false,
			Slug:    slug,
			Error:   fmt.Sprintf("Tag %s is not attached to this file", tag.String()),
		})
		return
	}

	list, err := tagsOfSlug(appDB, slug)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(FileTagsResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(FileTagsResponse{
		Success: true,
		Slug:    slug,
		Tags:    list,
		Message: fmt.Sprintf("Tag %s removed", tag.String()),
	})
}
