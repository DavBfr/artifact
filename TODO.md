# Artifact Server - TODO List

## Implemented

- [X] Single-file upload (REST + web UI) with progress
- [X] Short links (`/s/{slug}`)
- [X] Filename URLs (`/f/{filename}`)
- [X] Multi-arch Docker build
- [X] SQLite file records with soft deletes
- [X] File tags (`name:suffix`, `/t/{tag}` links, add/remove)
- [X] Paginated listing with search and sorting
- [X] Aggregate stats endpoint
- [X] Shared-token auth (`ART_API_TOKEN`)
- [X] Session tokens (`ART_SESSION_SECRET`, `token` command)
- [X] OIDC login
- [X] Permissions, roles and groups
- [X] S3-compatible storage backend
- [X] High availability (one writer, read-only replicas)
- [X] Health endpoint with replication lag
- [X] CORS, security headers, non-root container
- [X] Lock-down modes (`ART_NO_LISTING`, `ART_APPEND_ONLY`, `ART_NO_FILENAME_URL`, `ART_READ_ONLY`)

## Next

- [ ] Undelete / trash
- [ ] File integrity checks (SHA256)
- [ ] Bulk file operations
- [ ] API documentation (OpenAPI/Swagger)
- [ ] File rename
- [ ] Retention policy
  - [ ] Expiration / TTL
  - [ ] Uuntagged files
- [ ] API rate limiting
- [ ] Logging and audit trail (OTLP)
- [ ] Download statistics and analytics (OTLP)
- [ ] Webhook notifications
- [ ] Virus/malware scanning integration
- [ ] Dark mode support

## Known issues

- [ ] Superseded-file data loss on a failed upload: fixed for `fs`, untested on S3
- [ ] Tags referenced only by a pending upload are briefly unresolvable
- [ ] `ART_MAX_LIST_LIMIT` can truncate the UI's tag-owner lookup
