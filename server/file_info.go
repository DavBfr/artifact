package main

// fileInfoFromRecord builds the API-facing FileInfo from a db FileRecord and
// its tags. An empty tag list is rendered as [] rather than null so clients can
// always iterate the field.
func fileInfoFromRecord(rec FileRecord, tags []string) FileInfo {
	if tags == nil {
		tags = []string{}
	}
	return FileInfo{
		Name:     rec.DisplayName,
		Size:     rec.Size,
		Modified: rec.Modified,
		URL:      "/s/" + rec.Slug,
		MimeType: rec.MimeType,
		Tags:     tags,
	}
}
