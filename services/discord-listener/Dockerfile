FROM golang:1.26.0-alpine AS builder

WORKDIR /app

# Copy shared module
COPY shared/ ./shared/

# Copy service files
COPY services/discord-listener/ ./services/discord-listener/

# Download dependencies
WORKDIR /app/services/discord-listener
RUN go mod download

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/discord-listener ./cmd/

FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/discord-listener .
COPY LICENSE ./LICENSE

RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser && \
    chown -R appuser:appuser /app

USER appuser

EXPOSE 8086

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8086/health/live \
    && wget --no-verbose --tries=1 --spider http://localhost:8086/health/ready || exit 1

CMD ["./discord-listener"]
