# Build stage
FROM golang:1.27.4-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /build

# Copy shared module first (payment-service depends on it via a replace directive)
COPY shared /build/shared

# Copy service files
COPY services/payment-service /build/services/payment-service

WORKDIR /build/services/payment-service

RUN go mod download

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o payment-service ./cmd

# Runtime stage: distroless static (ADR-0092). No shell or package manager;
# ships CA certificates and tzdata. UID 1000 matches runAsUser in the k8s manifests.
FROM gcr.io/distroless/static-debian13

WORKDIR /app

COPY --from=builder /build/services/payment-service/payment-service .
COPY LICENSE ./LICENSE

USER 1000:1000

EXPOSE 8091

ENV PORT=8091
ENV LOG_LEVEL=info
ENV PATREON_MIN_TIER_CENTS=500
ENV PAYMENT_RECONCILE_INTERVAL=6h
ENV PATREON_TOKEN_REFRESH_BUFFER=24h

CMD ["./payment-service"]
