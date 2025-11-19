# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy shared module first
COPY shared/ ./shared/

# Copy service files
COPY services/event-collector/go.mod services/event-collector/go.sum* ./services/event-collector/
WORKDIR /app/services/event-collector

# Download dependencies
RUN go mod download

# Copy source code
COPY services/event-collector/ .

# Build the service
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /event-collector ./cmd/main.go

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy binary from builder
COPY --from=builder /event-collector .

# Expose port
EXPOSE 8090

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8090/health/live || exit 1

# Run the service
CMD ["./event-collector"]
