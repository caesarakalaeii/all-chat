# Build stage
FROM golang:1.27.4-alpine AS builder

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

# Runtime stage: distroless static (ADR-0092). No shell or package manager;
# ships CA certificates and tzdata. UID 1000 matches runAsUser in the k8s manifests.
FROM gcr.io/distroless/static-debian13

WORKDIR /app

COPY --from=builder /build/services/token-refresh-service/token-refresh-service .
COPY LICENSE ./LICENSE

USER 1000:1000

EXPOSE 8090

ENV PORT=8090
ENV LOG_LEVEL=info
ENV TOKEN_REFRESH_INTERVAL=5m
ENV TOKEN_REFRESH_BUFFER=30m
ENV TOKEN_REFRESH_BATCH_SIZE=100
ENV TOKEN_REFRESH_RETRY_ATTEMPTS=3

CMD ["./token-refresh-service"]
