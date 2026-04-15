# Build stage
FROM golang:1.25.6-alpine AS builder

WORKDIR /app

# Copy shared module
COPY shared/ ./shared/

# Copy service files
COPY services/share-service/ ./services/share-service/

# Download dependencies
WORKDIR /app/services/share-service
RUN go mod download

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/share-service ./cmd

# Runtime stage
FROM alpine:3.23

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/share-service .
COPY LICENSE ./LICENSE

# Change ownership
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8090

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8090/health/live || exit 1

# Run the binary
CMD ["./share-service"]
