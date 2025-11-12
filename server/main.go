package main

import (
	"fmt"

	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gorilla/mux"
)

func main() {
	// Configuration
	uploadFolder = os.Getenv("ART_UPLOAD_FOLDER")
	if uploadFolder == "" {
		uploadFolder = "/var/uploads"
	}

	staticFolder = os.Getenv("ART_STATIC_FOLDER")
	if staticFolder == "" {
		staticFolder = "./static"
	}

	maxFileSizeStr := os.Getenv("ART_MAX_FILE_SIZE")
	maxContentLength = parseSize(maxFileSizeStr, defaultMaxFileSize)

	apiToken = os.Getenv("ART_API_TOKEN")

	// Ensure upload directory exists
	if err := os.MkdirAll(uploadFolder, 0755); err != nil {
		log.Fatalf("Failed to create upload directory: %v", err)
	}

	// Setup router
	r := mux.NewRouter()

	// Apply CORS middleware to all routes
	r.Use(corsMiddleware)

	// API Routes - all under /api/ prefix
	r.HandleFunc("/api/health", healthCheckHandler).Methods("GET")
	r.HandleFunc("/api/files", listFilesHandler).Methods("GET")
	r.HandleFunc("/api/config", requireToken(getConfigHandler)).Methods("GET")
	r.HandleFunc("/api/upload", requireToken(uploadFileHandler)).Methods("POST")
	r.HandleFunc("/api/delete/{filename}", requireToken(deleteFileHandler)).Methods("DELETE")
	r.HandleFunc("/api/uploads/{filename}", serveFileHandler).Methods("GET")

	// Static files served as fallback (no /static/ prefix)
	// Check if static folder and index.html exist
	indexPath := filepath.Join(staticFolder, "index.html")
	if stat, err := os.Stat(staticFolder); err == nil && stat.IsDir() {
		if _, err := os.Stat(indexPath); err == nil {
			// Serve static files from root, falling back for any non-API routes
			fileServer := http.FileServer(http.Dir(staticFolder))
			r.PathPrefix("/").Handler(fileServer)
			log.Printf("Static folder: %s (serving at root)", staticFolder)
		} else {
			// Static folder exists but no index.html - return 404
			log.Printf("Static folder exists but no index.html found at: %s", indexPath)
			r.PathPrefix("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.NotFound(w, r)
			})
		}
	} else {
		// No static folder - return 404
		log.Printf("No static folder found at: %s (returning 404)", staticFolder)
		r.PathPrefix("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		})
	}

	// Start server
	port := os.Getenv("ART_PORT")
	if port == "" {
		port = "80"
	}

	addr := fmt.Sprintf("0.0.0.0:%s", port)
	log.Printf("Starting server on http://%s", addr)

	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
