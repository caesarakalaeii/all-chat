# Build stage
FROM golang:1.25.6-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

WORKDIR /build

# Copy shared module first
COPY shared /build/shared

# Copy service files
COPY services/twitch-listener /build/services/twitch-listener

WORKDIR /build/services/twitch-listener

# Download dependencies
RUN go mod download

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o twitch-listener ./cmd

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/services/twitch-listener/twitch-listener .

# Expose port
EXPOSE 8085

# Set environment variables
ENV PORT=8085
ENV LOG_LEVEL=info

# Run the application
CMD ["./twitch-listener"]
