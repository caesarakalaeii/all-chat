# Build stage
FROM golang:1.27.4-alpine AS builder

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

# kubectl for the read-only cluster tool: latest patch of the cluster's minor (1.36).
# kubectl must stay within one minor of the server, so bump this with k3s.
# Downloaded here because the runtime image has no shell; it is a static binary.
RUN wget -O /app/kubectl "https://dl.k8s.io/release/v1.36.6/bin/linux/amd64/kubectl" && \
    chmod 0755 /app/kubectl

# Runtime stage: distroless static (ADR-0092). No shell or package manager;
# ships CA certificates and tzdata. UID 1000 matches runAsUser in the k8s manifests.
# The tools exec kubectl directly (no shell), so it only needs to be on PATH.
FROM gcr.io/distroless/static-debian13

COPY --from=builder /app/kubectl /usr/local/bin/kubectl

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/support-bot .
COPY LICENSE ./LICENSE

USER 1000:1000

# kubectl caches under $HOME; point it at the writable tmpfs so the container can run
# with a read-only root filesystem.
ENV HOME=/tmp

EXPOSE 8094

CMD ["./support-bot"]
