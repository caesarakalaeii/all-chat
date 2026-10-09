# Build stage
FROM golang:1.27.0-alpine AS builder

WORKDIR /app

# Copy shared module
COPY shared/ ./shared/

# Copy service files
COPY services/instagram-listener/ ./services/instagram-listener/

# Download dependencies
WORKDIR /app/services/instagram-listener
RUN go mod download

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/instagram-listener ./cmd

# Runtime stage
FROM alpine:3.24

# Install CA certificates for HTTPS (Graph API is HTTPS-only)
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/instagram-listener .
COPY LICENSE ./LICENSE

# Create non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser && \
    chown -R appuser:appuser /app

USER appuser

# Expose health check port
EXPOSE 8100

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8100/health/live \
    && wget --no-verbose --tries=1 --spider http://localhost:8100/health/ready || exit 1

CMD ["./instagram-listener"]
