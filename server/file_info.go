package main

import (
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

	// Detect content type
	var contentType string
	if ext := strings.ToLower(filepath.Ext(filePath)); ext != "" {
		contentType = mime.TypeByExtension(ext)
	}

	return FileInfo{
		Name:     filepath.Base(filePath),
		Size:     stat.Size(),
		Modified: modTime.Format(time.RFC3339),
		URL:      "/api/uploads/" + filepath.Base(filePath),
		MimeType: contentType,
	}, nil
}
