# Build stage
FROM golang:1.26.3-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /build

# Copy shared module first (payment-service depends on it via a replace directive)
COPY shared /build/shared

# Copy service files
COPY services/payment-service /build/services/payment-service

WORKDIR /build/services/payment-service

RUN go mod download

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o payment-service ./cmd

# Final stage
FROM alpine:3.23

RUN apk --no-cache add ca-certificates tzdata && \
    addgroup -g 1000 appuser && adduser -D -u 1000 -G appuser appuser

WORKDIR /app

COPY --from=builder /build/services/payment-service/payment-service .
COPY LICENSE ./LICENSE

RUN chown -R appuser:appuser /app

USER appuser

EXPOSE 8091

ENV PORT=8091
ENV LOG_LEVEL=info
ENV PATREON_MIN_TIER_CENTS=500
ENV PAYMENT_RECONCILE_INTERVAL=6h
ENV PATREON_TOKEN_REFRESH_BUFFER=24h

CMD ["./payment-service"]
