# Build stage
FROM golang:1.26.0-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git

# Set working directory
WORKDIR /build

# Copy go.mod and go.sum from workspace root

# Copy shared module
COPY shared/ ./shared/

# Copy emote-service module
COPY services/emote-service/ ./services/emote-service/

# Set working directory to emote-service
WORKDIR /build/services/emote-service

# Download dependencies
RUN go mod download

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o emote-service ./cmd

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/services/emote-service/emote-service .

# Expose port
EXPOSE 8083

# Run the binary
CMD ["./emote-service"]
