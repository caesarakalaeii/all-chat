# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /build

# Copy go.mod and go.sum
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY *.go ./

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o dual-listener-test .

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/dual-listener-test .

# Create artifacts directory
RUN mkdir -p /artifacts

# Run as non-root user
RUN adduser -D -u 1000 testuser
USER testuser

ENTRYPOINT ["/app/dual-listener-test"]
