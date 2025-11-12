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

## Features

- **Simple File Upload**: Upload files via web interface or REST API
- **Modern Web UI**: Clean, responsive interface built with Flutter/Jaspr
- **RESTful API**: Full REST API for programmatic access
- **Token Authentication**: Secure your uploads with API tokens
- **File Management**: List, download, and delete files
- **Chunked Uploads**: Efficient handling of large files
- **Health Checks**: Built-in health endpoint for monitoring
- **Multi-architecture**: Supports both AMD64 and ARM64 platforms

## Use Cases

- **Development Teams**: Share build artifacts and assets
- **CI/CD Pipelines**: Store build outputs and test results
- **Content Distribution**: Simple file hosting for downloads

## Configuration

### Environment Variables

| Variable            | Description                                                        | Default        |
| ------------------- | ------------------------------------------------------------------ | -------------- |
| `ART_API_TOKEN`     | Authentication token for API access (required for uploads/deletes) | None           |
| `ART_PORT`          | Port to listen on                                                  | `8080`         |
| `ART_UPLOAD_FOLDER` | Directory to store uploaded files                                  | `/var/uploads` |
| `ART_STATIC_FOLDER` | Directory for static web files                                     | `/app/static`  |
| `ART_MAX_FILE_SIZE` | Maximum file size (e.g., "100M", "1G")                             | `100M`         |

### Volume Mounts

- `/var/uploads` - Persistent storage for uploaded files

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

## API Endpoints

### Health Check

```bash
GET /api/health
```

### Get Server Configuration

```bash
GET /api/config
```

### List Files

```bash
GET /api/files
Authorization: Bearer your-token-here
```

### Upload File

```bash
POST /api/upload
Authorization: Bearer your-token-here
Content-Type: multipart/form-data

# Example with curl:
curl -X POST http://localhost:8080/api/upload \
  -H "Authorization: Bearer your-token-here" \
  -F "file=@/path/to/your/file.pdf"

# Or with token in form data:
curl -X POST http://localhost:8080/api/upload \
  -F "file=@/path/to/your/file.pdf" \
  -F "token=your-token-here"
```

### Download File

```bash
GET /api/uploads/{filename}
```

### Delete File

```bash
DELETE /api/files/{filename}
Authorization: Bearer your-token-here
```

## Security

- **Authentication Required**: All file operations (upload, list, delete) require a valid API token
- **Non-root User**: Container runs as non-root user (UID 1000)
- **Read-only Downloads**: Public download endpoint (no authentication needed)
- **CORS Enabled**: Supports cross-origin requests for web applications

## Architecture

- **Backend**: Go with Gorilla Mux router
- **Frontend**: Flutter/Jaspr for server-side rendered web interface
- **Storage**: Local filesystem with configurable mount points
- **Size**: Minimal Alpine Linux base (~20-30MB compressed)

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
  "data": {
    "name": "file.pdf",
    "size": 1024000,
    "modified": "2025-11-11T10:30:00Z",
    "url": "/api/uploads/file.pdf"
  }
}
```
