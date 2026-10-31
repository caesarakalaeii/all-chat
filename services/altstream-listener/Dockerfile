# Build stage
FROM golang:1.27.3-alpine AS builder

WORKDIR /app

# Copy shared module
COPY shared/ ./shared/

# Copy service files
COPY services/altstream-listener/ ./services/altstream-listener/

# Download dependencies
WORKDIR /app/services/altstream-listener
RUN go mod download

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/altstream-listener ./cmd

# Runtime stage
FROM alpine:3.24

# Install CA certificates for HTTPS
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/altstream-listener .
COPY LICENSE ./LICENSE

# Create non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser && \
    chown -R appuser:appuser /app

USER appuser

# Expose health check port
EXPOSE 8101

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8101/health/live \
    && wget --no-verbose --tries=1 --spider http://localhost:8101/health/ready || exit 1

CMD ["./altstream-listener"]
