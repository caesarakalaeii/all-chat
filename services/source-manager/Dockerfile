# Build stage
FROM golang:1.26.1-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

WORKDIR /build

# Copy shared module first
COPY shared /build/shared

# Copy service files
COPY services/source-manager /build/services/source-manager

WORKDIR /build/services/source-manager

# Download dependencies
RUN go mod download

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o source-manager ./cmd

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/services/source-manager/source-manager .

# Expose port
EXPOSE 8088

# Set environment variables
ENV PORT=8088
ENV LOG_LEVEL=info

# Run the application
CMD ["./source-manager"]
