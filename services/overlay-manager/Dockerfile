# Build stage
FROM golang:1.25.4-alpine AS builder

WORKDIR /app

# Copy shared module
COPY shared/ ./shared/

# Copy service files
COPY services/overlay-manager/ ./services/overlay-manager/

# Download dependencies
WORKDIR /app/services/overlay-manager
RUN go mod download

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/overlay-manager ./cmd

# Runtime stage
FROM alpine:3.19

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/overlay-manager .

# Change ownership
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8082

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8082/health/live || exit 1

# Run the binary
CMD ["./overlay-manager"]
