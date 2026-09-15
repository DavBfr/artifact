package main

import (
	"database/sql"
	"fmt"

	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gorilla/mux"
	_ "modernc.org/sqlite"
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

	webPortal = parseBool(os.Getenv("ART_WEB_PORTAL"), true)
	noListing = parseBool(os.Getenv("ART_NO_LISTING"), false)
	appendOnly = parseBool(os.Getenv("ART_APPEND_ONLY"), false)
	noFilenameURL = parseBool(os.Getenv("ART_NO_FILENAME_URL"), false)

	maxListLimit = parseInt(os.Getenv("ART_MAX_LIST_LIMIT"), defaultMaxListLimit)
	if maxListLimit <= 0 {
		maxListLimit = defaultMaxListLimit
	}

	// Ensure upload directory exists
	if err := os.MkdirAll(uploadFolder, 0755); err != nil {
		log.Fatalf("Failed to create upload directory: %v", err)
	}

	// The sqlite db lives directly in uploadFolder (no separate volume). Its
	// absence beforehand means this is a fresh db, so any pre-existing flat
	// files should be adopted via the one-time legacy import.
	dbPath := filepath.Join(uploadFolder, dbFileName)
	_, statErr := os.Stat(dbPath)
	dbIsNew := os.IsNotExist(statErr)

	// Open the file record database. modernc.org/sqlite is pure Go (no CGO),
	// matching this project's static/scratch build. SQLite only allows one
	// writer at a time, so a single pooled connection serializes all access
	// instead of racing on SQLITE_BUSY.
	var err error
	appDB, err = sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer appDB.Close()
	appDB.SetMaxOpenConns(1)
	if _, err := appDB.Exec("PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000;"); err != nil {
		log.Fatalf("Failed to configure database: %v", err)
	}
	if err := initSchema(); err != nil {
		log.Fatalf("Failed to initialize database schema: %v", err)
	}

	// One-time migration: adopt any pre-existing flat files into the db-backed model
	if dbIsNew {
		if err := importLegacyUploads(); err != nil {
			log.Printf("Legacy upload import failed: %v", err)
		}
	}

	// Setup router
	r := mux.NewRouter()

	// Apply CORS middleware to all routes
	r.Use(corsMiddleware)

	// Apply security headers middleware to all routes
	r.Use(securityHeaders)

	// API Routes - all under /api/ prefix
	r.HandleFunc("/api/health", healthCheckHandler).Methods("GET")
	r.HandleFunc("/api/files", listFilesRouteHandler).Methods("GET")
	r.HandleFunc("/api/stats", statsRouteHandler).Methods("GET")
	r.HandleFunc("/api/config", getConfigHandler).Methods("GET")
	r.HandleFunc("/api/upload", requireToken(uploadFileHandler)).Methods("POST")
	r.HandleFunc("/api/delete/{slug}", requireToken(deleteFileHandler)).Methods("DELETE")
	r.HandleFunc("/s/{slug}", shortLinkHandler).Methods("GET")
	r.HandleFunc("/f/{filename}", filenameURLHandler).Methods("GET")

	// Static files served as fallback (no /static/ prefix)
	// Check if static folder and index.html exist
	indexPath := filepath.Join(staticFolder, "index.html")
	if !webPortal {
		log.Printf("Web portal disabled via ART_WEB_PORTAL (returning 404)")
		r.PathPrefix("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		})
	} else if stat, err := os.Stat(staticFolder); err == nil && stat.IsDir() {
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
