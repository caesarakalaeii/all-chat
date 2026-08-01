# Build stage
FROM golang:1.26.5-alpine AS builder

WORKDIR /app

# Copy shared module
COPY shared/ ./shared/

# Copy service files
COPY services/auth-service/ ./services/auth-service/

# Download dependencies (use go.mod directly, not go.work)
WORKDIR /app/services/auth-service
RUN go work vendor || go mod download

# Build auth-service
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/auth-service ./cmd

# Build key-rotator sweeper (D-03/D-06 — idempotent per-table re-encryption job)
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/key-rotator ./cmd/key-rotator

# Runtime stage
FROM alpine:3.24

RUN apk --no-cache add ca-certificates && \
    addgroup -g 1000 appuser && adduser -D -u 1000 -G appuser appuser

WORKDIR /app

COPY --from=builder /app/auth-service .
COPY --from=builder /app/key-rotator /app/key-rotator
COPY LICENSE ./LICENSE

RUN chown -R appuser:appuser /app

USER appuser

EXPOSE 8081

CMD ["./auth-service"]
