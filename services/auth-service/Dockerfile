# Build stage
FROM golang:1.27.4-alpine AS builder

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

# Build retention-sweeper (ADR-0089 — deletes rows past the privacy-policy retention periods)
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/retention-sweeper ./cmd/retention-sweeper

# Runtime stage: distroless static (ADR-0092). No shell or package manager;
# ships CA certificates and tzdata. UID 1000 matches runAsUser in the k8s manifests.
FROM gcr.io/distroless/static-debian13

WORKDIR /app

COPY --from=builder /app/auth-service .
COPY --from=builder /app/key-rotator /app/key-rotator
COPY --from=builder /app/retention-sweeper /app/retention-sweeper
COPY LICENSE ./LICENSE

USER 1000:1000

EXPOSE 8081

CMD ["./auth-service"]
