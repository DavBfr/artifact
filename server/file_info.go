package main

import (
	"os"
	"path/filepath"
	"time"
)

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
