# Multi-stage build for Go application (standalone, no nginx)
FROM golang:alpine AS builder

# Install build dependencies
RUN apk add --no-cache git

# Set working directory
WORKDIR /build

# Copy go mod files from server directory
COPY server/go.mod server/go.sum* ./

# Download dependencies
RUN go mod download

# Copy source code from server directory
COPY server/upload_server.go ./

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o upload_server .

FROM ghcr.io/cirruslabs/flutter:stable AS web_builder

# Set working directory
WORKDIR /app

# Copy pubspec files
COPY web/pubspec.yaml web/pubspec.lock* ./

# Get dependencies
RUN dart pub get

# Copy the web source code
COPY web/lib ./lib
COPY web/web ./web

# Prepare build files
RUN dart run build_runner build --delete-conflicting-outputs

# Build the web app
RUN dart run jaspr_cli:jaspr build

RUN mkdir -p /app/dist \
    && cp -r /app/build/jaspr/*.html /app/dist/ \
    && cp -r /app/build/jaspr/*.js /app/dist/ \
    && cp -r /app/build/jaspr/*.svg /app/dist/

# Final stage - minimal Alpine Linux
FROM alpine:latest

# Install ca-certificates for HTTPS support
RUN apk --no-cache add ca-certificates

# Create app user for security
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

# Set working directory
WORKDIR /app

# Copy the built Go binary from builder
COPY --from=builder /build/upload_server /app/upload_server

# Copy the built web files
# COPY --from=web_builder /app/dist/ ./static/
COPY ./static/ ./static/

# Create uploads directory and set permissions
RUN mkdir -p /var/uploads && \
    chown -R appuser:appuser /var/uploads /app

# Switch to non-root user
USER appuser

# Expose port 8080
EXPOSE 8080

# Set environment variables
ENV ART_UPLOAD_FOLDER=/var/uploads
ENV ART_STATIC_FOLDER=/app/static
ENV ART_PORT=8080

# Run the application
CMD ["/app/upload_server"]
