package main

import (
	"database/sql"
	"fmt"
	"io"

	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorilla/mux"
	_ "modernc.org/sqlite"
)

func main() {
	// Subcommands take over when present. They must not touch the upload folder
	// or the database, so this runs before any of the server setup below.
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "token":
			initSessionConfig()
			os.Exit(runTokenCommand(os.Args[2:]))
		case "help", "-h", "--help":
			printUsage(os.Stderr)
			os.Exit(0)
		default:
			fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
			printUsage(os.Stderr)
			os.Exit(2)
		}
	}

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

	// Session tokens (ART_SESSION_SECRET) are both what the OIDC login mints and
	// what the `token` command issues. Read before the OIDC config, which
	// refuses to enable itself without a signing key.
	initSessionConfig()
	initOIDCConfig()

	webPortal = parseBool(os.Getenv("ART_WEB_PORTAL"), true)
	noListing = parseBool(os.Getenv("ART_NO_LISTING"), false)
	appendOnly = parseBool(os.Getenv("ART_APPEND_ONLY"), false)
	noFilenameURL = parseBool(os.Getenv("ART_NO_FILENAME_URL"), false)

	storageKind = strings.ToLower(strings.TrimSpace(os.Getenv("ART_STORAGE")))
	if storageKind == "" {
		storageKind = storageFilesystem
	}
	if storageKind != storageFilesystem && storageKind != storageS3 {
		log.Fatalf("ART_STORAGE must be %q or %q (got %q)", storageFilesystem, storageS3, storageKind)
	}
	s3Bucket = os.Getenv("ART_S3_BUCKET")
	s3Prefix = os.Getenv("ART_S3_PREFIX")
	s3Region = os.Getenv("ART_S3_REGION")
	s3Endpoint = os.Getenv("ART_S3_ENDPOINT")
	// S3-compatible stores (MinIO and friends) need bucket-in-path addressing,
	// so an explicit endpoint implies path style unless told otherwise.
	s3PathStyle = parseBool(os.Getenv("ART_S3_PATH_STYLE"), s3Endpoint != "")

	maxListLimit = parseInt(os.Getenv("ART_MAX_LIST_LIMIT"), defaultMaxListLimit)
	if maxListLimit <= 0 {
		maxListLimit = defaultMaxListLimit
	}

	// Ensure the data directory exists. With s3 storage it only holds the
	// sqlite database (and its WAL sidecars), not the uploads themselves.
	if err := os.MkdirAll(uploadFolder, 0755); err != nil {
		log.Fatalf("Failed to create upload directory: %v", err)
	}

	if err := initStorage(); err != nil {
		log.Fatalf("Failed to configure %s storage: %v", storageKind, err)
	}
	log.Printf("Blob storage: %s", storageKind)

	// The sqlite db lives directly in uploadFolder (no separate volume).
	dbPath := filepath.Join(uploadFolder, dbFileName)

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
	if _, err := appDB.Exec("PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000; PRAGMA foreign_keys=ON;"); err != nil {
		log.Fatalf("Failed to configure database: %v", err)
	}
	if err := initSchema(); err != nil {
		log.Fatalf("Failed to initialize database schema: %v", err)
	}

	// Setup router
	r := mux.NewRouter()

	// Apply CORS middleware to all routes
	r.Use(corsMiddleware)

	// Apply security headers middleware to all routes
	r.Use(securityHeaders)

	// API Routes - all under /api/ prefix
	r.HandleFunc("/api/health", healthCheckHandler).Methods("GET")
	r.HandleFunc("/api/files", mayRequireToken(listFilesHandler)).Methods("GET")
	r.HandleFunc("/api/stats", mayRequireToken(statsHandler)).Methods("GET")
	r.HandleFunc("/api/config", getConfigHandler).Methods("GET")
	r.HandleFunc("/api/upload", requireToken(uploadFileHandler)).Methods("POST")
	r.HandleFunc("/api/delete/{slug}", requireToken(deleteFileHandler)).Methods("DELETE")
	r.HandleFunc("/api/uploads/{filename}", filenameURLHandler).Methods("GET")

	// File tag API Routes. Reading tags follows the listing policy
	// (public unless ART_NO_LISTING); changing them always needs the token.
	r.HandleFunc("/api/tags", mayRequireToken(listAllTagsHandler)).Methods("GET")
	r.HandleFunc("/api/tags/{slug}", mayRequireToken(listFileTagsHandler)).Methods("GET")
	r.HandleFunc("/api/tags/{slug}", requireToken(addFileTagsHandler)).Methods("POST")
	r.HandleFunc("/api/tags/{slug}/{tag:.+}", requireToken(removeFileTagHandler)).Methods("DELETE")

	// OIDC login. Registered unconditionally - the handlers answer 503 when OIDC
	// isn't configured, so enabling it is purely a matter of setting env vars.
	r.HandleFunc("/api/auth/login", oidcLoginHandler).Methods("GET")
	r.HandleFunc("/api/auth/callback", oidcCallbackHandler).Methods("GET")

	// File download API Routes
	// File download API Routes. HEAD is routed alongside GET so proxies, CDNs and
	// download managers can size a file without transferring it; the handlers
	// already answer it without reading the blob.
	r.HandleFunc("/s/{slug}", shortLinkHandler).Methods("GET", "HEAD")
	r.HandleFunc("/t/{tag:.+}", tagDownloadHandler).Methods("GET", "HEAD")
	r.HandleFunc("/f/{filename}", filenameURLHandler).Methods("GET", "HEAD")

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

// printUsage documents the server and its subcommands.
func printUsage(w io.Writer) {
	fmt.Fprint(w, `Artifact Server

Usage:
  upload_server                     Start the server
  upload_server token -sub <subject> [-ttl <duration>] [-name <name>]
                                    Mint an API token signed with
                                    ART_SESSION_SECRET
  upload_server help                 Show this message

Configuration is read from the environment; see the README for the full list.
`)
}
