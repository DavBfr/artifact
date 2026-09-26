# Artifact Server

A lightweight, standalone file upload server with a modern web interface. Built with Go for the backend and Dart/Flutter for the frontend, this container provides a simple yet powerful solution for storing and managing files via REST API.

## Quick Start

```bash
docker run -d \
  --name artifact-server \
  -p 8080:8080 \
  -v ./uploads:/var/uploads \
  -e ART_API_TOKEN=your-secret-token \
  davbfr/artifact:latest
```

Access the web interface at `http://localhost:8080`
Press `alt` to reveal the login button.

## Features

- **Simple File Upload**: Upload files via web interface or REST API
- **Modern Web UI**: Clean, responsive interface built with Flutter/Jaspr
- **RESTful API**: Full REST API for programmatic access
- **Token Authentication**: Secure your uploads with API tokens
- **File Management**: List, download, and delete files
- **Short Links**: Every uploaded file gets a permanent, non-enumerable `/s/{slug}` link for sharing/downloading
- **Filename URLs** *(optional)*: `/f/{filename}` always resolves to the latest version of that name; disable via `ART_NO_FILENAME_URL`
- **File Tags**: Attach docker-style `name:suffix` tags (`cat:latest`, `pets/cat:6.0`) at upload or later; every tag gets a stable `/t/{tag}` link that always resolves to the file it currently points at
- **Search & Sort**: Paginated file listing with server-side search and sorting (name, date, size)
- **Chunked Uploads**: Efficient handling of large files
- **Health Checks**: Built-in health endpoint for monitoring
- **Multi-architecture**: Supports both AMD64 and ARM64 platforms

## Use Cases

- **Development Teams**: Share build artifacts and assets
- **CI/CD Pipelines**: Store build outputs and test results
- **Content Distribution**: Simple file hosting for downloads

## Configuration

### Environment Variables

| Variable              | Description                                                                       | Default        |
| --------------------- | --------------------------------------------------------------------------------- | -------------- |
| `ART_API_TOKEN`       | Authentication token for API access (required for uploads/deletes)                | None           |
| `ART_PORT`            | Port to listen on                                                                 | `8080`         |
| `ART_UPLOAD_FOLDER`   | Directory to store uploaded files and the sqlite file-record database             | `/var/uploads` |
| `ART_STATIC_FOLDER`   | Directory for static web files                                                    | `/app/static`  |
| `ART_MAX_FILE_SIZE`   | Maximum file size (e.g., "100M", "1G")                                            | `100M`         |
| `ART_WEB_PORTAL`      | Serve the web interface. `false` disables it entirely (404s)                      | `true`         |
| `ART_NO_LISTING`      | `true` requires a valid API token to call `GET /api/files` and `GET /api/stats`   | `false`        |
| `ART_APPEND_ONLY`     | `true` keeps every upload as a separate file (no replacing) and disables deletion | `false`        |
| `ART_MAX_LIST_LIMIT`  | Hard cap on the number of files returned per `GET /api/files` request             | `500`          |
| `ART_NO_FILENAME_URL` | `true` disables `GET /f/{filename}` entirely (404s)                               | `false`        |

### Volume Mounts

- `/var/uploads` - Persistent storage for uploaded files (also holds the `artifact.db` sqlite file-record database)

## Docker Compose Example

```yaml
services:
  artifact-server:
    image: davbfr/artifact:latest
    container_name: artifact-server
    ports:
      - "8080:8080"
    volumes:
      - ./uploads:/var/uploads
    environment:
      - ART_API_TOKEN=your-secret-token-here
      - ART_MAX_FILE_SIZE=500M
    restart: unless-stopped
```

Create a `.env` file for environment variables:

```env
ART_API_TOKEN=your-secret-token
ART_MAX_FILE_SIZE=1G
```

Access the web interface at `http://localhost:8080`
Press `alt` to reveal the login button.

## API Endpoints

### Health Check

```bash
GET /api/health
```

### Get Server Configuration

```bash
GET /api/config
Authorization: Bearer your-token-here
```

Public - no token required - but the response depends on what's supplied:

| Field                   | Without a token                          | With a valid token |
| ----------------------- | ---------------------------------------- | ------------------ |
| `filename_urls_enabled` | included if "true" default false omitted | always included    |
| `max_list_limit`        | included only if `ART_NO_LISTING=false`  | always included    |
| `max_content_length`    | omitted                                  | included           |

An invalid (non-empty but wrong) token still returns `401`.

### Get Aggregate Stats

```bash
GET /api/stats
```

Returns `total_files`, `total_size`, and `last_upload` across all files (not just the current page). Requires a token only if `ART_NO_LISTING=true`.

### List Files

```bash
GET /api/files?limit=50&offset=0&search=report&order=-date
Authorization: Bearer your-token-here
```

Public by default; requires a token if `ART_NO_LISTING=true`. Query parameters (all optional):

| Param    | Description                                                             | Default    |
| -------- | ----------------------------------------------------------------------- | ---------- |
| `limit`  | Max files to return, capped by `ART_MAX_LIST_LIMIT` regardless of value | server max |
| `offset` | Number of matching files to skip                                        | `0`        |
| `search` | Case-insensitive substring match against the file name                  | none       |
| `order`  | One of `name`, `-name`, `date`, `-date`, `size`, `-size`                | `-date`    |

The response no longer includes a grand total - keep paging with increasing `offset` until `count` comes back `0`. Each entry also carries a `tags` array (empty, never null, when the file has no tags).

### Upload File

```bash
POST /api/upload
Authorization: Bearer your-token-here
Content-Type: multipart/form-data

# Example with curl:
curl -X POST http://localhost:8080/api/upload \
  -H "Authorization: Bearer your-token-here" \
  -F "file=@/path/to/your/file.pdf"
```

Each upload gets its own permanent short link. By default, uploading a file with the same name replaces the previous one (its short link stops working). With `ART_APPEND_ONLY=true`, uploads never replace an existing file - a new short link is created every time, even for a duplicate name.

#### Uploading with tags

Pass any number of `tags` form fields (repeat the field; its position relative to `file` doesn't matter):

```bash
curl -X POST http://localhost:8080/api/upload \
  -H "Authorization: Bearer your-token-here" \
  -F "tags=cat:latest" \
  -F "tags=pets/cat:6.0" \
  -F "file=@/path/to/cat.png"
```

A tag is `name:suffix`; a bare name means `:latest`. Names may be namespaced with `/`, but only the suffix is restricted to a single path-free segment, so `pets/cat:6.0` is valid and `cat:1/2` is not. Tags are case-sensitive and may contain letters, digits, `_`, `-` and `.` (`/` in the name only), up to 255 characters in total. An invalid tag fails the whole upload with `400` and stores nothing.

### Download File

```bash
GET /s/{slug}
```

Public, no authentication required. This is the `url` returned for each file by `/api/files` and `/api/upload`. Links are never reused, even after the file is deleted.

### Download File by Name (optional)

```bash
GET /f/{filename}
```

Public, no authentication required. Always resolves to the *latest* live upload with that display name (per the db), regardless of its short link slug - useful for a stable, human-readable URL that always points at the current version. Disabled entirely (404s) when `ART_NO_FILENAME_URL=true`. The UI only shows a "copy filename link" button when this is enabled (reported via `/api/config`'s `filename_urls_enabled`).

### Download File by Tag

```bash
GET /t/{tag}
```

Public, no authentication required. Resolves whatever file the tag currently points at, so a stable link like `/t/cat:latest` keeps working across re-uploads. Namespaced tags go straight in the path (`GET /t/pets/cat:6.0`); a bare name means `:latest` (`GET /t/cat` and `GET /t/cat:latest` are the same tag). Returns `404` for an unknown tag, or when the file it points at has been deleted.

### Tags

A file can carry any number of tags. A tag is globally unique because it always resolves to exactly one file: adding a tag that already belongs to another file *moves* it. Deleting a file deletes its tags, which frees those tag names for reuse (unlike short link slugs, which are never reused).

Uploading a file whose display name matches an existing file replaces it (see above). Any tag listed in the new upload moves to the new file, and the replaced file's remaining tags are deleted with it.

#### List All Tags

```bash
GET /api/tags?limit=50&offset=0&search=pets
Authorization: Bearer your-token-here
```

Public by default; requires a token if `ART_NO_LISTING=true`. Returns every tag attached to a live file, together with the file it resolves to.

| Param    | Description                                        | Default    |
| -------- | -------------------------------------------------- | ---------- |
| `limit`  | Max tags to return, capped by `ART_MAX_LIST_LIMIT` | server max |
| `offset` | Number of matching tags to skip                    | `0`        |
| `search` | Case-insensitive substring match on name or suffix | none       |

```json
{
  "success": true,
  "tags": [
    {
      "name": "pets/cat",
      "suffix": "6.0",
      "tag": "pets/cat:6.0",
      "slug": "aB3xQ",
      "url": "/t/pets/cat:6.0",
      "file_name": "cat.png",
      "created": "2026-09-26T12:00:00Z"
    }
  ],
  "count": 1
}
```

#### List a File's Tags

```bash
GET /api/tags/{slug}
Authorization: Bearer your-token-here
```

Public by default; requires a token if `ART_NO_LISTING=true`. `{slug}` is the file's short link id, not its display name. Returns `404` if the file is unknown, deleted, or still uploading.

```json
{ "success": true, "slug": "aB3xQ", "tags": ["cat:latest", "pets/cat:6.0"] }
```

#### Add Tags to a File

```bash
POST /api/tags/{slug}
Authorization: Bearer your-token-here
Content-Type: application/json

curl -X POST http://localhost:8080/api/tags/aB3xQ \
  -H "Authorization: Bearer your-token-here" \
  -H "Content-Type: application/json" \
  -d '{"tags":["cat:3.5","pets/cat:6.0"]}'
```

Always requires a token. Duplicate tags in one request are collapsed. Tags that already belong to another file are moved, and each one is reported in `moved` as `tag -> previous slug`. Returns `400` for an invalid tag or an empty list, `404` if the file is unknown or deleted.

```json
{
  "success": true,
  "slug": "aB3xQ",
  "tags": ["cat:3.5", "cat:latest", "pets/cat:6.0"],
  "moved": { "pets/cat:6.0": "Zt4pL" },
  "message": "One or more tags were moved from another file"
}
```

#### Remove a Tag from a File

```bash
DELETE /api/tags/{slug}/{tag}
Authorization: Bearer your-token-here

curl -X DELETE http://localhost:8080/api/tags/aB3xQ/cat:3.5 \
  -H "Authorization: Bearer your-token-here"
```

Always requires a token. Detaches only that tag from that file, leaving the tag name free for another file. Returns `404` if the tag is not attached to the file. Namespaced tags are sent as-is, e.g. `/api/tags/aB3xQ/pets/cat:6.0`.

### Delete File

```bash
DELETE /api/delete/{slug}
Authorization: Bearer your-token-here
```

The `{slug}` is the id from the file's short link (`url`), not its display name. Disabled entirely when `ART_APPEND_ONLY=true`. Deleting a file also deletes its tags, freeing those tag names for reuse.

## Security

- **Authentication Required**: Uploading and deleting files require a valid API token
- **Non-root User**: Container runs as non-root user (UID 10001)
- **Public Downloads**: Short links (`GET /s/{slug}`) require no authentication
- **Unguessable Short Links**: Slugs are generated with `crypto/rand`, not sequential or derived from the file name
- **CORS Enabled**: Supports cross-origin requests for web applications
- **Append-only Mode**: Set `ART_APPEND_ONLY=true` to keep every upload as a separate file and disable deletion entirely
- **Private Listing**: Set `ART_NO_LISTING=true` to require authentication for `GET /api/files` and `GET /api/stats`

## Architecture

- **Backend**: Go with Gorilla Mux router
- **Frontend**: Flutter/Jaspr for server-side rendered web interface
- **File Records**: SQLite (`modernc.org/sqlite`, pure Go, no CGO) tracks each upload's display name, short link slug, and physical storage path; deletions are soft (the row is kept so its slug can never be reused)
- **Tags**: A SQLite `tags` table keyed on `(name, suffix)` with a foreign key to `files(slug)` (`ON DELETE CASCADE`) maps each tag to one file; attaching a tag that already exists moves it instead of duplicating it
- **Storage**: Uploaded bytes are stored under `/var/uploads` in a slug-sharded layout (e.g. `ab/cdefgh...`), decoupled from the original filename
- **Size**: Minimal scratch-based image (~20-30MB compressed)

## Resource Usage

- **CPU**: Minimal (~1-2% idle)
- **Memory**: ~10-20MB base usage
- **Disk**: Depends on uploaded files

## Health Monitoring

The server includes a health check endpoint at `/api/health` that returns:

```json
{
  "status": "healthy",
  "service": "upload-server"
}
```

Use this for container health checks:

```yaml
healthcheck:
  test:
    [
      "CMD",
      "wget",
      "--quiet",
      "--tries=1",
      "--spider",
      "http://localhost:8080/api/health",
    ]
  interval: 30s
  timeout: 10s
  retries: 3
  start_period: 5s
```

## Response Format

All API responses follow this JSON structure:

```json
{
  "success": true,
  "message": "File uploaded successfully",
  "file": {
    "name": "file.pdf",
    "size": 1024000,
    "modified": "2025-11-11T10:30:00Z",
    "url": "/s/aB3xQ",
    "mime_type": "application/pdf",
    "tags": ["cat:latest"]
  },
  "replaced": false
}
```
