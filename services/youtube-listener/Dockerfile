# Build stage
FROM golang:1.25.4-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

WORKDIR /build

# Copy shared module first
COPY shared /build/shared

# Copy service files
COPY services/youtube-listener /build/services/youtube-listener

WORKDIR /build/services/youtube-listener

# Download dependencies
RUN go mod download

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o youtube-listener ./cmd

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/services/youtube-listener/youtube-listener .

# Expose port
EXPOSE 8086

# Set environment variables
ENV PORT=8086
ENV LOG_LEVEL=info

# Run the application
CMD ["./youtube-listener"]
