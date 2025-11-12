package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

const (
	defaultMaxFileSize = 100 * 1024 * 1024 // 100MB
	chunkSize          = 8 * 1024 * 1024   // 8MB chunks
)

// CORS middleware to allow requests from anywhere
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow from any origin
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400") // 24 hours

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

var (
	uploadFolder     string
	maxContentLength int64
	apiToken         string
	staticFolder     string
)

type FileInfo struct {
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	Modified string `json:"modified"`
	URL      string `json:"url"`
}

type Response struct {
	Success bool        `json:"success"`
	Error   string      `json:"error,omitempty"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

type ListFilesResponse struct {
	Success bool       `json:"success"`
	Files   []FileInfo `json:"files"`
	Count   int        `json:"count"`
	Error   string     `json:"error,omitempty"`
}

type UploadResponse struct {
	Success  bool     `json:"success"`
	Message  string   `json:"message,omitempty"`
	File     FileInfo `json:"file,omitempty"`
	Replaced bool     `json:"replaced"`
	Error    string   `json:"error,omitempty"`
}

type ConfigResponse struct {
	Success          bool   `json:"success"`
	MaxContentLength int64  `json:"max_content_length"`
	Error            string `json:"error,omitempty"`
}

type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

type DeleteResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

func parseSize(s string, defaultSize int64) int64 {
	if s == "" {
		return defaultSize
	}

	s = strings.TrimSpace(strings.ToUpper(s))
	units := map[string]int64{
		"K":  1024,
		"KB": 1024,
		"M":  1024 * 1024,
		"MB": 1024 * 1024,
		"G":  1024 * 1024 * 1024,
		"GB": 1024 * 1024 * 1024,
	}

	for unit, multiplier := range units {
		if strings.HasSuffix(s, unit) {
			valueStr := s[:len(s)-len(unit)]
			if value, err := strconv.ParseFloat(valueStr, 64); err == nil {
				return int64(value * float64(multiplier))
			}
			return defaultSize
		}
	}

	if value, err := strconv.ParseInt(s, 10, 64); err == nil {
		return value
	}

	return defaultSize
}

func requireToken(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")

		// Also check for token in form data for curl compatibility
		if token == "" {
			token = r.FormValue("token")
		}

		if token == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(Response{
				Success: false,
				Error:   "Authentication required. Provide token in Authorization header or token form field.",
			})
			return
		}

		// Remove 'Bearer ' prefix if present
		token = strings.TrimPrefix(token, "Bearer ")

		if token != apiToken {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(Response{
				Success: false,
				Error:   "Invalid authentication token",
			})
			return
		}

		next(w, r)
	}
}

func getFileInfo(filePath string) (FileInfo, error) {
	stat, err := os.Stat(filePath)
	if err != nil {
		return FileInfo{}, err
	}

	modTime := stat.ModTime()
	return FileInfo{
		Name:     filepath.Base(filePath),
		Size:     stat.Size(),
		Modified: modTime.Format(time.RFC3339),
		URL:      "/api/uploads/" + filepath.Base(filePath),
	}, nil
}

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(HealthResponse{
		Status:  "healthy",
		Service: "upload-server",
	})
}

func listFilesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	files := []FileInfo{}

	entries, err := os.ReadDir(uploadFolder)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ListFilesResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			filePath := filepath.Join(uploadFolder, entry.Name())
			fileInfo, err := getFileInfo(filePath)
			if err == nil {
				files = append(files, fileInfo)
			}
		}
	}

	// Sort by modification time (newest first)
	sort.Slice(files, func(i, j int) bool {
		return files[i].Modified > files[j].Modified
	})

	json.NewEncoder(w).Encode(ListFilesResponse{
		Success: true,
		Files:   files,
		Count:   len(files),
	})
}

func getConfigHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ConfigResponse{
		Success:          true,
		MaxContentLength: maxContentLength,
	})
}

func uploadFileHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Parse multipart form with max memory
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(UploadResponse{
			Success: false,
			Error:   "Failed to parse form: " + err.Error(),
		})
		return
	}

	file, handler, err := r.FormFile("file")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(UploadResponse{
			Success: false,
			Error:   "No file provided",
		})
		return
	}
	defer file.Close()

	if handler.Filename == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(UploadResponse{
			Success: false,
			Error:   "No file selected",
		})
		return
	}

	// Check file size
	if handler.Size > maxContentLength {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		json.NewEncoder(w).Encode(UploadResponse{
			Success: false,
			Error:   fmt.Sprintf("File too large. Maximum size is %d bytes", maxContentLength),
		})
		return
	}

	// Secure the filename (basic version)
	filename := filepath.Base(filepath.Clean(handler.Filename))
	destPath := filepath.Join(uploadFolder, filename)

	// Check if file already exists
	replaced := false
	if _, err := os.Stat(destPath); err == nil {
		replaced = true
	}

	// Create destination file
	dst, err := os.Create(destPath)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(UploadResponse{
			Success: false,
			Error:   "Failed to create file: " + err.Error(),
		})
		return
	}
	defer dst.Close()

	// Copy file with chunked reading for efficient memory usage
	buffer := make([]byte, chunkSize)
	if _, err := io.CopyBuffer(dst, file, buffer); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(UploadResponse{
			Success: false,
			Error:   "Failed to save file: " + err.Error(),
		})
		return
	}

	// Get file info for response
	fileInfo, err := getFileInfo(destPath)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(UploadResponse{
			Success: false,
			Error:   "Failed to get file info: " + err.Error(),
		})
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(UploadResponse{
		Success:  true,
		Message:  "File uploaded successfully",
		File:     fileInfo,
		Replaced: replaced,
	})
}

func deleteFileHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	filename := vars["filename"]

	// Secure the filename
	filename = filepath.Base(filepath.Clean(filename))
	filePath := filepath.Join(uploadFolder, filename)

	// Check if file exists
	stat, err := os.Stat(filePath)
	if err != nil || stat.IsDir() {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(DeleteResponse{
			Success: false,
			Error:   "File not found",
		})
		return
	}

	// Delete the file
	if err := os.Remove(filePath); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(DeleteResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(DeleteResponse{
		Success: true,
		Message: fmt.Sprintf("File %s deleted successfully", filename),
	})
}

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
	if apiToken == "" {
		apiToken = "default-token"
	}

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
