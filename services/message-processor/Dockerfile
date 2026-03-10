# Build stage
FROM golang:1.26.0-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

WORKDIR /build

# Copy shared module first
COPY shared /build/shared

# Copy service files
COPY services/message-processor /build/services/message-processor

WORKDIR /build/services/message-processor

# Download dependencies
RUN go mod download

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o message-processor ./cmd

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/services/message-processor/message-processor .

# Expose port
EXPOSE 8087

# Set environment variables
ENV PORT=8087
ENV LOG_LEVEL=info

# Run the application
CMD ["./message-processor"]
