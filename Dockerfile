# =============================================================================
# Multi-stage Dockerfile for TG-Manager
# =============================================================================
# This Dockerfile provides two targets:
# 1. development - Hot reload with Air, includes dev tools
# 2. production  - Optimized, minimal final image
# =============================================================================

# -----------------------------------------------------------------------------
# Stage 1: Base Image
# -----------------------------------------------------------------------------
FROM golang:1.26-alpine AS base

# Install system dependencies
RUN apk add --no-cache \
    git \
    make \
    curl \
    bash \
    ca-certificates \
    tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download && go mod verify

# -----------------------------------------------------------------------------
# Stage 2: Development (with hot reload)
# -----------------------------------------------------------------------------
FROM base AS development

# Install Air for hot reload
RUN go install github.com/cosmtrek/air@latest

# Install other dev tools
RUN go install github.com/golang/mock/mockgen@latest && \
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Copy application source
COPY . .

# Create directories for logs
RUN mkdir -p /var/log/tg-manager

# Expose ports
EXPOSE 8080 9090

# Default command (will be overridden by docker-compose)
CMD ["air", "-c", ".air.toml"]

# -----------------------------------------------------------------------------
# Stage 3: Builder (compile for production)
# -----------------------------------------------------------------------------
FROM base AS builder

# Build arguments
ARG APP_VERSION=dev
ARG BUILD_TIME
ARG GIT_COMMIT

# Copy application source
COPY . .

# Build flags for optimization
ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

# Build web application
RUN go build \
    -ldflags="-w -s" \
    -o /bin/tg-manager-web \
    ./cmd/web/main.go

# Build worker application
RUN go build \
    -ldflags="-w -s" \
    -o /bin/tg-manager-worker \
    ./cmd/worker/main.go

# Verify binaries
RUN chmod +x /bin/tg-manager-web /bin/tg-manager-worker

# -----------------------------------------------------------------------------
# Stage 4: Production Web
# -----------------------------------------------------------------------------
FROM alpine:3.19 AS production-web

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    curl

# Create non-root user
RUN addgroup -g 1000 tgmanager && \
    adduser -D -u 1000 -G tgmanager tgmanager

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /bin/tg-manager-web /app/tg-manager-web

# Create directories
RUN mkdir -p /var/log/tg-manager && \
    chown -R tgmanager:tgmanager /app /var/log/tg-manager

# Switch to non-root user
USER tgmanager

# Expose ports
EXPOSE 8080 9090

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

# Run application
CMD ["/app/tg-manager-web"]

# -----------------------------------------------------------------------------
# Stage 5: Production Worker
# -----------------------------------------------------------------------------
FROM alpine:3.19 AS production-worker

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata

# Create non-root user
RUN addgroup -g 1000 tgmanager && \
    adduser -D -u 1000 -G tgmanager tgmanager

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /bin/tg-manager-worker /app/tg-manager-worker

# Create directories
RUN mkdir -p /var/log/tg-manager && \
    chown -R tgmanager:tgmanager /app /var/log/tg-manager

# Switch to non-root user
USER tgmanager

# Run application
CMD ["/app/tg-manager-worker"]

# -----------------------------------------------------------------------------
# Stage 6: Production All-in-One (Web + Worker in single container)
# -----------------------------------------------------------------------------
FROM alpine:3.19 AS production

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    curl \
    supervisor

# Create non-root user
RUN addgroup -g 1000 tgmanager && \
    adduser -D -u 1000 -G tgmanager tgmanager

# Set working directory
WORKDIR /app

# Copy binaries from builder
COPY --from=builder /bin/tg-manager-web /app/tg-manager-web
COPY --from=builder /bin/tg-manager-worker /app/tg-manager-worker

# Copy supervisor config
COPY configs/supervisor/supervisord.conf /etc/supervisord.conf

# Create directories
RUN mkdir -p /var/log/tg-manager /var/log/supervisor && \
    chown -R tgmanager:tgmanager /app /var/log/tg-manager /var/log/supervisor

# Expose ports
EXPOSE 8080 9090

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

# Run supervisor
CMD ["/usr/bin/supervisord", "-c", "/etc/supervisord.conf"]

# =============================================================================
# Build Instructions:
# =============================================================================
#
# Development (with hot reload):
#   docker build --target development -t tg-manager:dev .
#   docker run -p 8080:8080 -v $(pwd):/app tg-manager:dev
#
# Production Web:
#   docker build --target production-web -t tg-manager-web:latest \
#     --build-arg APP_VERSION=1.0.0 \
#     --build-arg BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ") \
#     --build-arg GIT_COMMIT=$(git rev-parse HEAD) .
#
# Production Worker:
#   docker build --target production-worker -t tg-manager-worker:latest \
#     --build-arg APP_VERSION=1.0.0 \
#     --build-arg BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ") \
#     --build-arg GIT_COMMIT=$(git rev-parse HEAD) .
#
# Production All-in-One:
#   docker build --target production -t tg-manager:latest \
#     --build-arg APP_VERSION=1.0.0 \
#     --build-arg BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ") \
#     --build-arg GIT_COMMIT=$(git rev-parse HEAD) .
#
# Multi-platform build (for production):
#   docker buildx build --platform linux/amd64,linux/arm64 \
#     --target production-web -t tg-manager-web:latest .
#
# =============================================================================
