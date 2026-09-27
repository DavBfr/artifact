package main

// fileInfoFromRecord builds the API-facing FileInfo from a db FileRecord and
// its tags. The tags field is omitted from the JSON entirely when the file has
// none, so a nil slice is fine here.
func fileInfoFromRecord(rec FileRecord, tags []string) FileInfo {
	return FileInfo{
		Name:     rec.DisplayName,
		Size:     rec.Size,
		Modified: rec.Modified,
		URL:      "/s/" + rec.Slug,
		MimeType: rec.MimeType,
		Tags:     tags,
	}
}
