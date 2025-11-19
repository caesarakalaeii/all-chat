# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy shared module first
COPY shared/ ./shared/

# Copy service files
COPY services/clip-manager/go.mod services/clip-manager/go.sum* ./services/clip-manager/
WORKDIR /app/services/clip-manager

# Download dependencies
RUN go mod download

# Copy source code
COPY services/clip-manager/ .

# Build the service
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /clip-manager ./cmd/main.go

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy binary from builder
COPY --from=builder /clip-manager .

# Expose port
EXPOSE 8091

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8091/health/live || exit 1

# Run the service
CMD ["./clip-manager"]
