# Build stage
FROM golang:1.25.6-alpine AS builder

WORKDIR /app

# Copy modules this service depends on via replace directives
COPY shared/ ./shared/
COPY services/message-processor/ ./services/message-processor/

# Copy service files
COPY services/moderation-service/ ./services/moderation-service/

# Download dependencies
WORKDIR /app/services/moderation-service
RUN go mod download

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/moderation-service ./cmd

# Runtime stage
FROM alpine:3.23

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/moderation-service .
COPY LICENSE ./LICENSE

# Change ownership
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8092

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8092/health/live || exit 1

# Run the binary
CMD ["./moderation-service"]
