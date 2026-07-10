# Build stage
FROM golang:1.25.6-alpine AS builder

WORKDIR /app

# Copy modules this service depends on via replace directives
COPY shared/ ./shared/

# Copy service files
COPY services/support-bot/ ./services/support-bot/

# Download dependencies
WORKDIR /app/services/support-bot
RUN go mod download

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/support-bot ./cmd

# Runtime stage
FROM alpine:3.23

# ca-certificates for HTTPS (GitHub, Grafana, the local LLM if TLS).
RUN apk --no-cache add ca-certificates

# kubectl for the read-only cluster tool (match cluster server version v1.33.5).
# busybox wget (built into alpine) downloads it; no curl dependency needed.
RUN wget -O /usr/local/bin/kubectl "https://dl.k8s.io/release/v1.33.5/bin/linux/amd64/kubectl" && \
    chmod 0755 /usr/local/bin/kubectl

# Create non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/support-bot .
COPY LICENSE ./LICENSE

RUN chown -R appuser:appuser /app

USER appuser

# kubectl caches under $HOME; point it at the writable tmpfs so the container can run
# with a read-only root filesystem.
ENV HOME=/tmp

EXPOSE 8094

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8094/health/live || exit 1

CMD ["./support-bot"]
