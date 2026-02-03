# Build stage
FROM golang:1.25.6-alpine AS builder

WORKDIR /app

# Copy shared module
COPY shared/ ./shared/

# Copy service files
COPY services/api-gateway/ ./services/api-gateway/

# Download dependencies
WORKDIR /app/services/api-gateway
RUN go mod download

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/api-gateway ./cmd

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/api-gateway .

# Copy static files
COPY static/ ./static/

# Expose port
EXPOSE 8080

# Set environment variables
ENV PORT=8080
ENV LOG_LEVEL=info

# Run the application
CMD ["./api-gateway"]
