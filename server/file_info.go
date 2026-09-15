package main

import (
	"log"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func getFileInfo(filePath string) (FileInfo, error) {
	stat, err := os.Stat(filePath)
	if err != nil {
		return FileInfo{}, err
	}

	modTime := stat.ModTime()
	name := filepath.Base(filePath)

	// Detect content type
	var contentType string
	if ext := strings.ToLower(filepath.Ext(filePath)); ext != "" {
		contentType = mime.TypeByExtension(ext)
	}

	// Short link creation is best-effort: failures shouldn't break file listing/upload
	var shortURL string
	if slug, err := getOrCreateShortLink(name); err != nil {
		log.Printf("Failed to get or create short link for %s: %v", name, err)
	} else {
		shortURL = "/s/" + slug
	}

	return FileInfo{
		Name:     name,
		Size:     stat.Size(),
		Modified: modTime.Format(time.RFC3339),
		URL:      "/api/uploads/" + name,
		MimeType: contentType,
		ShortURL: shortURL,
	}, nil
}
