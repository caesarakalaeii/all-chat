# Build stage
FROM golang:1.25.6-alpine AS builder

WORKDIR /app

# Copy modules this service depends on via replace directives
COPY shared/ ./shared/

# Copy service files
COPY services/youtube-quota-monitor/ ./services/youtube-quota-monitor/

# Download dependencies
WORKDIR /app/services/youtube-quota-monitor
RUN go mod download

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/youtube-quota-monitor ./cmd

# Runtime stage
FROM alpine:3.23

# ca-certificates for HTTPS; tzdata because the monitor resolves America/Los_Angeles
# (YouTube quota resets at midnight Pacific).
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/youtube-quota-monitor .
COPY LICENSE ./LICENSE

# Change ownership
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8093

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8093/health/live || exit 1

# Run the binary
CMD ["./youtube-quota-monitor"]
