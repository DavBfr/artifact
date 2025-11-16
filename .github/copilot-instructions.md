# Artifact Server - AI Coding Agent Instructions

## Architecture Overview

**Artifact Server** is a containerized file upload service with three distinct components:

1. **Go Backend** (`server/`): Standalone REST API server using gorilla/mux
2. **Jaspr Web UI** (`web/`): Client-rendered Dart/Jaspr web application (compiles to JS)
3. **Docker Multi-stage Build**: Compiles both components into a single Alpine image

Key architectural decisions:

- **No database**: Files stored directly in filesystem (`/var/uploads`)
- **Stateless authentication**: Single shared token (`ART_API_TOKEN`) via Bearer auth
- **Client-side rendering**: Jaspr compiles Dart to JavaScript for web execution
- **Custom Bulma components**: Hand-rolled Jaspr components mimicking Bulma CSS patterns
- **Alt-key auth reveal**: Login UI hidden until user presses Alt key (see `key_listener.dart`)

## Project Structure

```
server/           # Go backend - one handler per file pattern
  main.go         # Router setup, env config, static file serving
  *_handler.go    # Each API endpoint in separate file
  token.go        # requireToken() middleware for protected routes

web/lib/
  main.dart       # Entry point, wraps App in providers
  widgets/        # Page-level components (app.dart, files_list.dart, etc)
  bulma/          # Custom Bulma-style components for Jaspr
  models/         # API client (api.dart) and JSON models (api_models.dart)
  utils/          # Browser localStorage wrapper (token_storage.dart)

flt/              # Experimental Flutter desktop version (not production)
```

## Development Workflows

### Running Locally

```bash
# Backend only (serves on :9080, expects frontend at web/build/jaspr/)
cd server && go run .

# Frontend dev (requires dart run build_runner first)
cd web && dart run jaspr_cli:jaspr serve

# Full stack with Docker Compose watch mode
docker compose watch  # Auto-rebuilds on file changes
```

### Building Web UI

**Critical**: Jaspr requires code generation before building:

```bash
cd web
dart run build_runner build --delete-conflicting-outputs  # Generates *.g.dart files
dart run jaspr_cli:jaspr build -O4                        # Compiles to JS
```

Output lands in `web/build/jaspr/` (index.html, main.dart.js, styles.css)

### Go Backend Patterns

- **File handlers**: One Go file per endpoint (e.g., `upload_file.go` for `/api/upload`)
- **Global config**: Variables like `uploadFolder`, `apiToken` set in `main.go` from env vars
- **Protected routes**: Wrap handlers with `requireToken()` (see `main.go` lines 45-47)
- **CORS middleware**: Applied to all routes in `main.go` (line 42)

## Jaspr/Dart Frontend Conventions

### Component Architecture

- **Providers at root**: `NotificationMessengerProvider` and `DialogManagerProvider` wrap entire app (see `main.dart`)
- **Stateful components**: Use `State<T>` pattern similar to Flutter (e.g., `AppState` in `app.dart`)
- **@client annotation**: Mark components that should run client-side (currently only `App` uses this)

### API Communication

```dart
// API client initialized with base URL and optional token
_api = ArtifactApiClient.base(authToken: storedToken);

// Upload with progress tracking (uses XMLHttpRequest directly)
await _api.uploadFile(
  file: webFile,
  onProgress: (sent, total) => setState(() {...}),
);
```

### Token Storage

Token persisted in browser `localStorage` via `TokenStorage` utility:

```dart
TokenStorage.saveToken(context, token);  // Store
final token = TokenStorage.getToken(context);  // Retrieve
TokenStorage.removeToken(context);  // Logout
```

### Custom Bulma Components

Components in `web/lib/bulma/` implement Bulma CSS patterns as Jaspr widgets:

- **Notifications**: `NotificationMessenger.of(context).showNotification(...)` - auto-dismiss after duration
- **Dialogs**: `DialogManager.of(context).showDialog<T>(...)` - returns `Future<T?>` like Flutter
- **Pattern**: Most components are stateless wrappers around `div()` with Bulma classes

Example from `notifications.dart`:

```dart
BulmaNotification.error('Message', title: 'Error')  // Creates styled notification
```

### Code Generation

Models in `models/api_models.dart` use `json_serializable`:

- Add `part 'api_models.g.dart';` directive
- Annotate classes with `@JsonSerializable()`
- Run `dart run build_runner build` to generate `*.g.dart` files
- Never edit `.g.dart` files manually

## Docker Build Process

Multi-stage Dockerfile:

1. **builder**: Compiles Go binary from `server/`
2. **web_builder**: Runs Dart build pipeline (pub get → build_runner → jaspr build → minification)
3. **final**: Alpine image with Go binary + compiled JS/CSS in `/app/static/`

Build for multiple architectures:

```bash
docker buildx bake --push  # Uses platforms from compose.yml
```

## API Authentication Model

- **Public endpoints**: `/api/health`, `/api/files`, `/api/uploads/{filename}`
- **Protected endpoints**: `/api/config`, `/api/upload`, `/api/delete/{filename}`
- **Token validation**: `requireToken()` middleware checks `Authorization: Bearer <token>` header
- **No token configured**: Server returns 401 if `ART_API_TOKEN` env var is empty
- **Frontend behavior**: App loads file list publicly, hides upload/delete UI until authenticated

## Common Patterns

### Error Handling in Go

```go
if err != nil {
    w.WriteHeader(http.StatusInternalServerError)
    json.NewEncoder(w).Encode(Response{Success: false, Error: err.Error()})
    return
}
```

### State Management in Jaspr

```dart
setState(() {
  _files = filesResponse.files;  // Triggers rebuild like Flutter
});
```

### File Operations

- **Upload**: Multipart form with `file` field, returns `FileInfo` with URL
- **Delete**: DELETE `/api/delete/{filename}` with auth
- **Download**: GET `/api/uploads/{filename}` (no auth required)
- **List**: GET `/api/files` returns all files sorted by name

## Testing & Validation

No formal test suite exists. Validation approach:

1. Build Docker image: `docker buildx bake`
2. Run compose: `docker compose up`
3. Test web UI at http://localhost:9080
4. Verify API with curl (see README.md examples)

## Key Files to Reference

- `server/main.go`: Complete routing and middleware setup
- `web/lib/widgets/app.dart`: Main app state, auth flow, and data loading
- `web/lib/models/api.dart`: All API client methods with error handling
- `Dockerfile`: Full build pipeline from source to production image
- `compose.yml`: Development watch configuration
