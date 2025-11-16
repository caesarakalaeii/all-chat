# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy go.mod and go.sum for all modules referenced in go.work
COPY go.work go.work.sum* ./
COPY shared/go.mod shared/go.sum ./shared/
COPY services/api-gateway/go.mod services/api-gateway/go.sum ./services/api-gateway/
COPY services/auth-service/go.mod services/auth-service/go.sum ./services/auth-service/
COPY services/emote-service/go.mod services/emote-service/go.sum ./services/emote-service/
COPY services/kick-listener/go.mod services/kick-listener/go.sum* ./services/kick-listener/
COPY services/message-processor/go.mod services/message-processor/go.sum ./services/message-processor/
COPY services/overlay-manager/go.mod services/overlay-manager/go.sum ./services/overlay-manager/
COPY services/source-manager/go.mod services/source-manager/go.sum ./services/source-manager/
COPY services/twitch-listener/go.mod services/twitch-listener/go.sum ./services/twitch-listener/
COPY services/youtube-listener/go.mod services/youtube-listener/go.sum ./services/youtube-listener/

# Download dependencies
WORKDIR /app/services/kick-listener
RUN go mod download

# Copy source code
WORKDIR /app
COPY shared/ ./shared/
COPY services/kick-listener/ ./services/kick-listener/

# Build the service
WORKDIR /app/services/kick-listener
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o /app/kick-listener \
    ./cmd/

# Runtime stage
FROM alpine:latest

# Install CA certificates for HTTPS
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/kick-listener .

# Create non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser && \
    chown -R appuser:appuser /app

USER appuser

# Expose health check port
EXPOSE 8089

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8089/health/live || exit 1

CMD ["./kick-listener"]
