# Build stage
FROM golang:1.26.0-alpine AS builder

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
FROM alpine:3.21

RUN apk --no-cache add ca-certificates tzdata && \
    addgroup -g 1000 appuser && adduser -D -u 1000 -G appuser appuser

WORKDIR /app

COPY --from=builder /build/services/token-refresh-service/token-refresh-service .
COPY LICENSE ./LICENSE

RUN chown -R appuser:appuser /app

USER appuser

EXPOSE 8090

ENV PORT=8090
ENV LOG_LEVEL=info
ENV TOKEN_REFRESH_INTERVAL=5m
ENV TOKEN_REFRESH_BUFFER=10m
ENV TOKEN_REFRESH_BATCH_SIZE=100
ENV TOKEN_REFRESH_RETRY_ATTEMPTS=3

CMD ["./token-refresh-service"]
