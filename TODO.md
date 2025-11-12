# Artifact Server - TODO List

This document outlines potential improvements and feature additions for the Artifact Server project.

## 🎨 User Experience Enhancements

### 1. Add search/filter functionality for files

**Priority:** High
**Effort:** Low

Implement client-side search to filter files by name. Add search bar in `files_list.dart` that filters the displayed files based on user input. This is especially useful when there are many files uploaded.

### 2. Add file sorting options

**Priority:** High
**Effort:** Low

Add ability to sort files by name, size, or date (ascending/descending). Add sorting dropdown or clickable column headers in `files_list.dart`. Currently files are sorted in `upload_server.go` but only by name ascending.

### 3. Implement pagination for large file lists

**Priority:** Medium
**Effort:** Medium

When there are many files, implement pagination or virtual scrolling to improve performance. Add pagination controls or infinite scroll in `files_list.dart`. Consider adding pagination support to the backend API as well.

### 4. Add bulk file operations

**Priority:** Medium
**Effort:** High

Allow users to select multiple files for batch operations (download as zip, bulk delete). Add checkboxes to file items, selection state management, and bulk action buttons. Requires backend support for zip creation.

### 5. Add file preview capabilities

**Priority:** Medium
**Effort:** Medium

Implement preview modal for common file types (images, PDFs, text files, code). Add modal component and file type detection. Could use browser native rendering or libraries like pdf.js for PDFs.

## 📁 File Management Features

### 6. Add file rename functionality

**Priority:** Medium
**Effort:** Low

Allow authenticated users to rename files after upload. Add rename API endpoint in `upload_server.go`, and add rename button/dialog in `files_list.dart`.

### 7. Implement file metadata and tags

**Priority:** Medium
**Effort:** High

Allow users to add descriptions, tags, or custom metadata to files. Store metadata in JSON files alongside uploads or in a simple database. Add UI for editing metadata and searching by tags.

### 8. Add file versioning support

**Priority:** Low
**Effort:** High

Instead of replacing files with same name, keep versions. Store files with version suffix and maintain version history. Add API to list versions and UI to view/download specific versions.

### 9. Implement storage quota and limits

**Priority:** High
**Effort:** Medium

Add configurable storage quota per instance. Track total storage used and display in `stats_card.dart`. Add environment variable for max storage and prevent uploads when quota exceeded.

### 10. Add file expiration/TTL feature

**Priority:** Low
**Effort:** Medium

Allow setting expiration dates on files (auto-delete after N days). Add optional TTL parameter to upload endpoint, background job to clean expired files, and display expiration date in UI.

## 🔗 Sharing & Collaboration

### 11. Implement file sharing links

**Priority:** High
**Effort:** Medium

Generate temporary, shareable links for files without requiring authentication. Add share link generation endpoint, token-based access, and share button in UI with copy-to-clipboard functionality.

### 12. Add download statistics and analytics

**Priority:** Low
**Effort:** Medium

Track download counts, last accessed time, and access patterns. Store stats in metadata or database. Display in file details and add analytics dashboard showing most downloaded files.

### 13. Implement folder/directory support

**Priority:** Medium
**Effort:** High

Add ability to organize files into folders. Update backend to support directory structure, add folder creation/navigation UI, and breadcrumb navigation. Update API endpoints to handle paths.

### 14. Add compression support

**Priority:** Low
**Effort:** Medium

Add option to compress files on upload (gzip/brotli) to save storage space. Add decompression on download. Add toggle in upload UI and compression ratio stats.

### 15. Implement multi-user support with roles

**Priority:** Low
**Effort:** Very High

Add user management with different permission levels (admin, uploader, viewer). Requires user database, registration/login system, and role-based access control in API endpoints.

## 🔧 Backend & Infrastructure

### 16. Add API rate limiting

**Priority:** High
**Effort:** Medium

Implement rate limiting to prevent API abuse. Add middleware in `upload_server.go` to track requests per IP/token and return 429 Too Many Requests when limits exceeded.

### 17. Add webhook notifications

**Priority:** Low
**Effort:** Low

Allow configuring webhooks to notify external services on file upload/delete events. Add webhook configuration via environment variables and HTTP POST to webhook URLs on events.

### 18. Implement S3-compatible storage backend

**Priority:** Medium
**Effort:** High

Add option to store files in S3/MinIO instead of local filesystem. Add storage backend abstraction layer and S3 client configuration via environment variables for cloud deployments.

### 19. Add virus/malware scanning integration

**Priority:** Medium
**Effort:** High

Integrate with ClamAV or similar to scan uploaded files. Add optional virus scanning step in upload flow, quarantine suspicious files, and display scan status in UI.

## 💻 Developer Experience

### 20. Improve error handling and user feedback

**Priority:** High
**Effort:** Medium

Add more detailed error messages throughout the application. Improve error handling in API client, add retry logic for failed uploads, and show more informative error notifications to users.

### 21. Add dark mode support

**Priority:** Low
**Effort:** Low

Implement dark theme toggle using Bulma's dark mode classes or custom CSS. Add theme switcher in navbar and persist preference in localStorage. Update all components to support dark mode.

### 22. Add clipboard paste upload support

**Priority:** Low
**Effort:** Low

Allow users to paste images/files from clipboard directly. Add paste event listener in `upload_section.dart` that handles clipboard data and uploads files.

### 23. Implement automatic upload retry on failure

**Priority:** Medium
**Effort:** Low

Add retry mechanism for failed uploads with exponential backoff. Update `api.dart` uploadFile method to retry on network errors, and show retry status in UI.

### 24. Add file integrity checks (checksums)

**Priority:** Medium
**Effort:** Medium

Calculate and verify file checksums (SHA256) to ensure integrity. Store checksums in metadata, verify on download, and display in file details UI.

### 25. Implement API documentation (OpenAPI/Swagger)

**Priority:** Medium
**Effort:** Medium

Generate OpenAPI specification for the REST API. Add Swagger UI endpoint to serve interactive API documentation. Document all endpoints, request/response schemas, and authentication.

### 26. Add logging and audit trail

**Priority:** High
**Effort:** Medium

Implement structured logging for all operations. Log uploads, downloads, deletes with timestamps and user info. Add configurable log levels and optional log export to external systems.

## 🎯 Polish & Performance

### 27. Improve mobile responsiveness

**Priority:** Medium
**Effort:** Low

Test and improve UI on mobile devices. Ensure drag-and-drop works on touch devices, optimize layout for small screens, and improve button/touch target sizes for mobile users.

### 28. Add file deduplication

**Priority:** Low
**Effort:** High

Detect duplicate files using content hashing and store only once. Add reference counting for duplicates and display to users when uploading duplicates.

### 29. Implement upload queue and concurrent uploads

**Priority:** Medium
**Effort:** Medium

Allow multiple files to be queued and uploaded concurrently. Add queue management in `app.dart`, show queue status, and allow users to cancel queued uploads.

### 30. Add keyboard shortcuts

**Priority:** Low
**Effort:** Low

Implement useful keyboard shortcuts (e.g., Ctrl+U for upload, Delete for delete file, Ctrl+F for search). Extend `key_listener.dart` with more shortcuts and show shortcut help dialog.

---

## Priority Legend

- **High:** Core functionality or security-related
- **Medium:** Enhances usability significantly
- **Low:** Nice-to-have features

## Effort Legend

- **Low:** 1-2 days
- **Medium:** 3-5 days
- **High:** 1-2 weeks
- **Very High:** 2+ weeks

## Quick Wins (High Priority + Low Effort)

1. Add search/filter functionality for files (#1)
2. Add file sorting options (#2)
3. Implement storage quota and limits (#9)
4. Implement file sharing links (#11)
5. Add API rate limiting (#16)
6. Improve error handling and user feedback (#20)
7. Add logging and audit trail (#26)
