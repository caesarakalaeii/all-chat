# Build stage
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

WORKDIR /build

# Copy service files (build from repo root, not service directory)
COPY . /build

WORKDIR /build/services/youtube-listener-innertube

# Download dependencies
RUN go mod download

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o youtube-listener-innertube ./cmd

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/services/youtube-listener-innertube/youtube-listener-innertube .

# Expose port for health checks
EXPOSE 8080

# Set default environment variables
ENV PORT=8080
ENV LOG_LEVEL=info

# Run the application
CMD ["./youtube-listener-innertube"]
