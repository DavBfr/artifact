package main

import (
	"net/http"
	"os"
	"path/filepath"

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

	// Serve the file with proper content type detection
	http.ServeContent(w, r, filename, stat.ModTime(), file)
}
