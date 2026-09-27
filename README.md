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
- **File Tags**: Attach docker-style `name:suffix` tags (`cat:latest`, `pets/cat:6.0`) at upload or later; every tag gets a stable `/t/{tag}` link that always resolves to the file it currently points at. The UI shows a file's tags inline (click one to copy its link) and keeps full tag management, along with every other detail, in the file properties dialog
- **Search & Sort**: Paginated file listing with server-side search and sorting (name, date, size)
- **Chunked Uploads**: Efficient handling of large files
- **Pluggable Storage**: Keep uploads on local disk (the default) or in an S3 bucket, including S3-compatible stores like MinIO
- **Health Checks**: Built-in health endpoint for monitoring
- **Multi-architecture**: Supports both AMD64 and ARM64 platforms

## Use Cases

- **Development Teams**: Share build artifacts and assets
- **CI/CD Pipelines**: Store build outputs and test results
- **Content Distribution**: Simple file hosting for downloads

## Configuration

### Environment Variables

| Variable                 | Description                                                                                                     | Default                               |
| ------------------------ | --------------------------------------------------------------------------------------------------------------- | ------------------------------------- |
| `ART_API_TOKEN`          | Authentication token for API access (required for uploads/deletes)                                              | None                                  |
| `ART_SESSION_SECRET`     | Signing key for session tokens. Enables token auth on its own (min. 32 chars)                                   | None                                  |
| `ART_SESSION_TTL`        | Lifetime of a session token minted by an OIDC login (e.g. `12h`, `30d`)                                         | `12h`                                 |
| `ART_OIDC_ISSUER`        | OIDC issuer URL; set with the client id to enable provider login                                                | None                                  |
| `ART_OIDC_CLIENT_ID`     | OIDC client id registered with the provider                                                                     | None                                  |
| `ART_OIDC_CLIENT_SECRET` | OIDC client secret; omit for a PKCE-only client                                                                 | None                                  |
| `ART_OIDC_SCOPES`        | Space-separated OIDC scopes                                                                                     | `openid profile email`                |
| `ART_OIDC_REDIRECT_URL`  | Redirect URI sent to the provider; must match its registration                                                  | `<scheme>://<host>/api/auth/callback` |
| `ART_PORT`               | Port to listen on                                                                                               | `8080`                                |
| `ART_UPLOAD_FOLDER`      | Data directory: holds the sqlite file-record database, plus the uploaded files themselves when `ART_STORAGE=fs` | `/var/uploads`                        |
| `ART_STORAGE`            | Where uploaded files live: `fs` (local disk) or `s3`                                                            | `fs`                                  |
| `ART_S3_BUCKET`          | Bucket for uploaded files; required when `ART_STORAGE=s3`                                                       | None                                  |
| `ART_S3_PREFIX`          | Key prefix inside the bucket (e.g. `artifacts`)                                                                 | none (bucket root)                    |
| `ART_S3_REGION`          | Region for the endpoint; defaults to the SDK's own resolution                                                   | None                                  |
| `ART_S3_ENDPOINT`        | Custom endpoint for S3-compatible stores (MinIO, Ceph, R2)                                                      | None (AWS)                            |
| `ART_S3_PATH_STYLE`      | Bucket-in-path addressing, needed by most S3-compatible stores                                                  | `true` when `ART_S3_ENDPOINT` is set  |
| `ART_STATIC_FOLDER`      | Directory for static web files                                                                                  | `/app/static`                         |
| `ART_MAX_FILE_SIZE`      | Maximum file size (e.g., "100M", "1G")                                                                          | `100M`                                |
| `ART_WEB_PORTAL`         | Serve the web interface. `false` disables it entirely (404s)                                                    | `true`                                |
| `ART_NO_LISTING`         | `true` requires a valid API token to call `GET /api/files` and `GET /api/stats`                                 | `false`                               |
| `ART_APPEND_ONLY`        | `true` keeps every upload as a separate file (no replacing) and disables deletion                               | `false`                               |
| `ART_MAX_LIST_LIMIT`     | Hard cap on the number of files returned per `GET /api/files` request                                           | `500`                                 |
| `ART_NO_FILENAME_URL`    | `true` disables `GET /f/{filename}` entirely (404s)                                                             | `false`                               |

### Volume Mounts

- `/var/uploads` - Holds the `artifact.db` sqlite file-record database, and with `ART_STORAGE=fs`
  (the default) the uploaded files too. With S3 storage nothing else is needed locally, so the
  directory only needs enough space for the database.

## Storage Backends

Uploaded files are kept either on local disk or in an S3 bucket.

### Local disk (default)

`ART_STORAGE=fs` stores each upload under `ART_UPLOAD_FOLDER` in a slug-sharded layout
(`ab/cdefgh...`), decoupled from its display name. Nothing changes for existing deployments.

### S3 (`ART_STORAGE=s3`)

```bash
ART_STORAGE=s3
ART_S3_BUCKET=my-artifacts          # object keys are <prefix>/<slug-sharded key>
ART_S3_PREFIX=artifacts             # optional
ART_S3_REGION=eu-west-1             # optional; the SDK resolves it otherwise
ART_S3_ENDPOINT=https://s3.example.com   # optional, for non-AWS stores
AWS_ACCESS_KEY_ID=...               # or any other credential source below
AWS_SECRET_ACCESS_KEY=...
```

- **Credentials** come from the standard AWS chain - environment, shared config, or a
  container/instance role - so nothing secret has to live in the app's own configuration.
- **MinIO and other S3-compatible stores** work through `ART_S3_ENDPOINT`, which also switches on
  path-style addressing (bucket in the path) by default. Override with `ART_S3_PATH_STYLE=false`
  only if your endpoint needs virtual-host addressing.
- **Nothing is staged on local disk**: uploads stream straight to the bucket through the SDK's
  transfer manager, in buffered parts, and the size cap is still enforced while reading. Downloads
  read ranges straight from the object, so byte ranges and resumable transfers keep working.
- **The database always stays local.** SQLite needs a local file, so `ART_UPLOAD_FOLDER` is still
  required with S3 - it just holds `artifact.db` instead of the uploads. That also means the record
  database is still a single-writer store: S3 makes the *files* shareable between instances, not the
  metadata.
- **Migrating an existing deployment** is a straight copy, because an object key is the
  `storage_key` the database already stores:

  ```bash
  aws s3 sync /var/uploads s3://my-artifacts/artifacts/ --exclude 'artifact.db*'
  ```

  Uploaded files whose records are marked deleted can be left behind or swept separately; only rows
  the database still lists as live are served.

## Authentication

Uploading, deleting and tagging always require a credential; listing and downloading are public
unless `ART_NO_LISTING=true`.

Two kinds of credential are accepted, as `Authorization: Bearer <token>` or a bare
`Authorization: <token>`:

1. **`ART_API_TOKEN`** - a shared secret, as before.
2. **A session token** - an HS256 JWT signed with `ART_SESSION_SECRET`. This is what the
   browser receives after an OIDC login, and what the `token` command below mints.

`ART_SESSION_SECRET` is a complete authentication configuration on its own: with it set,
uploads and deletes demand a valid session token instead of reporting that no token is
configured. It must be at least 32 characters, and tokens carry `iss=artifact-server` and
`aud=artifact-api`, so a secret shared with another service cannot be used to forge one.

### Issuing API tokens (no OIDC required)

```bash
# Valid for 30 days
docker exec artifact-server /app/upload_server token -sub ci -ttl 30d

# Valid for ART_SESSION_TTL, 12h by default
docker exec artifact-server /app/upload_server token -sub ci

# Never expires - a bearer credential that outlives everything but a secret rotation
docker exec artifact-server /app/upload_server token -sub build-agent -ttl 0
```

Only the token goes to stdout, so it can be captured directly:

```bash
TOKEN=$(docker exec artifact-server /app/upload_server token -sub ci -ttl 720h)
curl -H "Authorization: Bearer $TOKEN" -F "file=@build.zip" http://localhost:8080/api/upload
```

**Rotating `ART_SESSION_SECRET` invalidates every session and every minted token at once.**
There is no revocation list and no way to log out a single user, so rotation is the lever for
an emergency logoff.

### OpenID Connect

Setting both `ART_OIDC_ISSUER` and `ART_OIDC_CLIENT_ID` (alongside the required
`ART_SESSION_SECRET`) switches the web UI to provider login. Register this exact redirect URI
with the provider first - login fails with a redirect-URI error otherwise:

```text
http://localhost:8080/api/auth/callback
```

When OIDC is configured:

- The UI's login button (revealed with `alt`) sends the browser to `/api/auth/login`, which
  redirects to the provider with PKCE, a `state` and a `nonce`. The provider returns to
  `/api/auth/callback`. The token form is not offered in the web UI in this mode, although the
  API still accepts `ART_API_TOKEN`.
- The callback verifies the `id_token` and mints a session token for the browser
  (`ART_SESSION_TTL`, 12h by default). The provider's own access token is discarded: only
  tokens this server signed are accepted.
- Any identity the provider authenticates gets full upload and delete rights. There is no group
  or claim check, so restrict access at the provider - for example with a client that only a
  chosen set of users can authenticate against.
- The navbar shows who is signed in, decoded from the token for display only.
- An unreachable provider does not stop the server starting: downloads and the API keep
  working, and `/api/auth/login` answers `503` until discovery succeeds.

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
- **Storage**: Behind a storage interface with two backends. `fs` keeps uploaded bytes under `/var/uploads` in a slug-sharded layout (e.g. `ab/cdefgh...`); `s3` streams them to an object bucket under the same key shape, so several instances can serve one set of files. Either way the key is decoupled from the original filename
- **Size**: Minimal scratch-based image (~17MB, ~20MB with S3 support)

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
