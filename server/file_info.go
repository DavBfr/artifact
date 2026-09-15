package main

// fileInfoFromRecord builds the API-facing FileInfo from a db FileRecord.
func fileInfoFromRecord(rec FileRecord) FileInfo {
	return FileInfo{
		Name:     rec.DisplayName,
		Size:     rec.Size,
		Modified: rec.Modified,
		URL:      "/s/" + rec.Slug,
		MimeType: rec.MimeType,
	}
}
