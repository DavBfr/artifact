package main

import (
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
)

func serveFileHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	filename := vars["filename"]

	// Secure the filename
	filename = filepath.Base(filepath.Clean(filename))
	filePath := filepath.Join(uploadFolder, filename)

	// Check if file exists
	stat, err := os.Stat(filePath)
	if err != nil || stat.IsDir() {
		http.NotFound(w, r)
		return
	}

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		http.Error(w, "Failed to open file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// Set security headers
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("X-XSS-Protection", "1; mode=block")

	// Detect content type: try by extension first, then sniff up to 512 bytes
	var contentType string
	if ext := strings.ToLower(filepath.Ext(filename)); ext != "" {
		contentType = mime.TypeByExtension(ext)
	}

	// If unknown, read up to 512 bytes to sniff the content type
	if contentType == "" {
		var buf [512]byte
		n, _ := file.Read(buf[:])
		contentType = http.DetectContentType(buf[:n])
		// Reset read pointer so ServeContent can read from start
		_, _ = file.Seek(0, 0)
	}

	// Check if content type is dangerous and serve as binary instead
	if isDangerousContentType(contentType) {
		contentType = "application/octet-stream"
	}

	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}

	// Set Content-Length header
	w.Header().Set("Content-Length", strconv.FormatInt(stat.Size(), 10))

	// Set Content-Disposition to attachment to prompt download
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")

	// Serve the file with proper support for ranges and conditional requests
	http.ServeContent(w, r, filename, stat.ModTime(), file)
}

// isDangerousContentType checks if a content type could be harmful if executed in a browser
func isDangerousContentType(contentType string) bool {
	// List of potentially dangerous content types that should be served as binary
	dangerousTypes := []string{
		"text/javascript",
		"application/javascript",
		"application/x-javascript",
		"text/html",
		"application/html",
		"text/xml",
		"application/xml",
		"application/xhtml+xml",
		"image/svg+xml",
		"application/vnd.ms-excel",
		"application/x-msexcel",
		"application/x-excel",
		"application/x-mspowerpoint",
		"application/powerpoint",
		"application/x-powerpoint",
		"application/vnd.ms-powerpoint",
		"application/vnd.openxmlformats-officedocument",
		"application/x-zip-compressed",
		"application/zip",
	}

	for _, dangerous := range dangerousTypes {
		if strings.HasPrefix(strings.ToLower(contentType), strings.ToLower(dangerous)) {
			return true
		}
	}
	return false
}
