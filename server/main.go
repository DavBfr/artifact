package main

import (
	"context"
	"errors"
	"fmt"
	"io"

	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/gorilla/mux"
	_ "modernc.org/sqlite"
)

func main() {
	// Subcommands take over when present. These must not touch the upload folder
	// or the database, so they run before any of the server setup below.
	command := ""
	if len(os.Args) > 1 {
		command = os.Args[1]
	}
	switch command {
	case "token":
		initSessionConfig()
		os.Exit(runTokenCommand(os.Args[2:]))
	case "help", "-h", "--help":
		printUsage(os.Stderr)
		os.Exit(0)
	case "", "restore-db":
		// restore-db needs the upload folder and the S3 configuration read below,
		// so it is dispatched once those are known.
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		printUsage(os.Stderr)
		os.Exit(2)
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
	//
	// The role configuration sits between the two: whether the provider is asked
	// for group membership depends on whether ART_OIDC_GROUPS uses it.
	initSessionConfig()
	initRolesConfig()
	initOIDCConfig()
	logAuthzConfig()

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

	// Read-only replicas. A replica has no volume of its own: it downloads the
	// writer's database from the snapshot store and serves that, so its bucket
	// credentials only ever need read access.
	readOnly = parseBool(os.Getenv("ART_READ_ONLY"), false)
	dbReplica = parseBool(os.Getenv("ART_DB_REPLICA"), false)
	if dbReplica {
		readOnly = true
		if storageKind != storageS3 {
			log.Fatalf("ART_DB_REPLICA needs ART_STORAGE=%s: a replica has no local blobs to serve", storageS3)
		}
	}

	// ART_DB_RESTORE says what an absent local database means. Unset is the safe
	// default: the server refuses to start when a snapshot exists, because an
	// empty database would be published over it within one backup interval.
	dbRestoreMode = strings.ToLower(strings.TrimSpace(os.Getenv("ART_DB_RESTORE")))
	switch dbRestoreMode {
	case "", dbRestoreAuto, dbRestoreIgnore:
	default:
		log.Fatalf("ART_DB_RESTORE must be %q or %q (got %q)", dbRestoreAuto, dbRestoreIgnore, dbRestoreMode)
	}

	dbBackupPrefix = strings.Trim(strings.TrimSpace(os.Getenv("ART_DB_BACKUP_PREFIX")), "/")
	if dbBackupPrefix == "" {
		dbBackupPrefix = defaultDBBackupPrefix
	}
	dbBackupDelay = durationEnv("ART_DB_BACKUP_DELAY", 0)
	dbPollInterval = durationEnv("ART_DB_POLL_INTERVAL", defaultDBPollInterval)

	maxListLimit = parseInt(os.Getenv("ART_MAX_LIST_LIMIT"), defaultMaxListLimit)
	if maxListLimit <= 0 {
		maxListLimit = defaultMaxListLimit
	}

	if command == "restore-db" {
		os.Exit(runRestoreCommand(os.Args[2:]))
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

	// The snapshot store comes first: bootDatabase asks it whether a missing
	// local database is a lost volume or simply a first boot, and a replica has
	// to download one before it can open anything.
	if err := initSnapshotStore(context.Background()); err != nil {
		log.Fatalf("Failed to configure database snapshots: %v", err)
	}
	if err := bootDatabase(); err != nil {
		log.Fatalf("Failed to open the database: %v", err)
	}
	defer closeDatabase()

	// backgroundContext stops the snapshot loops on the way out, which is what
	// gives the publisher its last chance to publish.
	backgroundContext, stopBackground = context.WithCancel(context.Background())
	defer stopBackground()

	var background sync.WaitGroup
	runSnapshotPublisher(&background)
	runSnapshotReplica(&background)

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
	// Mutations are refused on a read-only instance. Authentication and then
	// permission are checked first, so an unauthenticated caller still gets the
	// same 401 it always would, and a credential without the permission gets a
	// 403 that says so rather than the instance's role.
	//
	// Uploading and adding tags look like a single permission each, but their
	// effects depend on the payload, so they are checked per effect inside the
	// handler (see uploadFileHandler and addFileTagsHandler).
	r.HandleFunc("/api/upload", requireToken(requirePermission(permFileCreate, rejectWhenReadOnly(uploadFileHandler)))).Methods("POST")
	r.HandleFunc("/api/delete/{slug}", requireToken(requirePermission(permFileDelete, rejectWhenReadOnly(deleteFileHandler)))).Methods("DELETE")
	r.HandleFunc("/api/uploads/{filename}", filenameURLHandler).Methods("GET")

	// File tag API Routes. Reading tags follows the listing policy
	// (public unless ART_NO_LISTING); changing them needs the permission for the
	// change - and adding a tag that already belongs to another file is also a
	// removal from that file, which addFileTagsHandler checks for.
	r.HandleFunc("/api/tags", mayRequireToken(listAllTagsHandler)).Methods("GET")
	r.HandleFunc("/api/tags/{slug}", mayRequireToken(listFileTagsHandler)).Methods("GET")
	r.HandleFunc("/api/tags/{slug}", requireToken(requirePermission(permTagAdd, rejectWhenReadOnly(addFileTagsHandler)))).Methods("POST")
	r.HandleFunc("/api/tags/{slug}/{tag:.+}", requireToken(requirePermission(permTagRemove, rejectWhenReadOnly(removeFileTagHandler)))).Methods("DELETE")

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

	server := &http.Server{Addr: addr, Handler: r}

	signalCtx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	shutdown := make(chan struct{})
	go func() {
		defer close(shutdown)
		<-signalCtx.Done()
		log.Printf("Shutting down")

		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("Shutdown did not complete cleanly: %v", err)
		}
	}()

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("Server failed: %v", err)
	}

	// Stop the snapshot loops, giving the publisher its last chance to publish,
	// and only then close the database they read through.
	stopBackground()
	background.Wait()
	<-shutdown
	closeDatabase()
	log.Printf("Stopped")
}

// printUsage documents the server and its subcommands.
func printUsage(w io.Writer) {
	fmt.Fprint(w, `Artifact Server

Usage:
  upload_server                     Start the server
  upload_server token -sub <subject> [-ttl <duration>] [-name <name>]
                                    Mint an API token signed with
                                    ART_SESSION_SECRET
  upload_server restore-db [-force]
                                    Replace the local database with the
                                    published snapshot
  upload_server help                 Show this message

Configuration is read from the environment; see the README for the full list.
`)
}
