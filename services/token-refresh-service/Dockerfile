# Build stage
FROM golang:1.25.6-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

WORKDIR /build

# Copy shared module first
COPY shared /build/shared

# Copy auth-service (for OAuth providers)
COPY services/auth-service /build/services/auth-service

# Copy service files
COPY services/token-refresh-service /build/services/token-refresh-service

WORKDIR /build/services/token-refresh-service

# Download dependencies
RUN go mod download

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o token-refresh-service ./cmd

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS and tzdata for timezone support
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/services/token-refresh-service/token-refresh-service .

# Expose port
EXPOSE 8090

# Set environment variables
ENV PORT=8090
ENV LOG_LEVEL=info
ENV TOKEN_REFRESH_INTERVAL=5m
ENV TOKEN_REFRESH_BUFFER=10m
ENV TOKEN_REFRESH_BATCH_SIZE=100
ENV TOKEN_REFRESH_RETRY_ATTEMPTS=3

# Run the application
CMD ["./token-refresh-service"]
