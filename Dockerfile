FROM ghcr.io/cirruslabs/flutter:stable AS web_builder

# Set working directory
WORKDIR /app

RUN apt update && apt install -y minify librsvg2-bin optipng scour

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
RUN dart run jaspr_cli:jaspr build -O4

RUN mkdir -p dist \
    && cp build/jaspr/*.html dist/ \
    && cp build/jaspr/*.css dist/ \
    && cp build/jaspr/*.js dist/ \
    && cp build/jaspr/*.svg dist/

RUN \
    for f in $(find dist -name '*.html' -o -name '*.css' -o -name '*.js' -a -not -name '*.dart.js'); do \
    minify -o $f $f; \
    done

RUN rsvg-convert dist/logo.svg -o dist/favicon.png -w 256 -h 256

RUN optipng -o7 dist/favicon.png

RUN \
    for f in build/jaspr/*.svg; do \
    base=$(basename $f); \
    scour --enable-id-stripping --enable-comment-stripping \
    --remove-descriptive-elements --shorten-ids --indent=none \
    -i $f -o dist/$base; \
    done

FROM golang:alpine AS builder

ARG TARGETARCH
ARG CSP_VERSION=v0.1.3

# Install build dependencies
RUN apk add --no-cache git curl

RUN curl -fsSL "https://github.com/DavBfr/csp/releases/download/${CSP_VERSION}/csp_linux_${TARGETARCH}.tar.gz" | tar -xz -C /usr/local/bin csp

# Set working directory
WORKDIR /build

# Copy go mod files from server directory
COPY server/go.mod server/go.sum* ./

# Download dependencies
RUN go mod download

# Copy source code from server directory
COPY server/ ./

# Build the application
RUN \
    --mount=type=bind,from=web_builder,target=/web \
    CSP_HASHED=$(csp -csp "default-src 'none'; script-src 'self'; style-src 'self' https://cdn.jsdelivr.net https://cdnjs.cloudflare.com; img-src 'self'; font-src 'self' https://cdnjs.cloudflare.com; connect-src 'self'; frame-src 'none'; frame-ancestors 'none'; object-src 'none'; base-uri 'self'; form-action 'self'; media-src 'self'; manifest-src 'self'; worker-src 'self'; upgrade-insecure-requests; style-src-attr 'unsafe-inline'" -include-external -heuristics $(find /web/app/dist/ -name '*.html' -print)) && \
    CGO_ENABLED=0 GOOS=linux go build -a -ldflags "-extldflags \"-static\" -s -w -X \"main.cspHeader=${CSP_HASHED}\"" -installsuffix cgo -o upload_server .

# Final stage - minimal Alpine Linux
FROM alpine:latest

LABEL org.opencontainers.image.vendor="NfetDotNet"
LABEL org.opencontainers.image.authors="dev.nfet.net@gmail.com"
LABEL org.opencontainers.image.licenses="Apache-2.0"
LABEL org.opencontainers.image.title="Artifact Server"
LABEL org.opencontainers.image.description="A simple file upload server with web interface."

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
COPY --from=web_builder /app/dist/ ./static/

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
