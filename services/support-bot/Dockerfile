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

# Claude Code CLI, the unmodified native binary, for the subscription OAuth path
# (CLAUDE_CODE_OAUTH_TOKEN, ADR-0093). Pinned and checksum-verified against the
# release manifest (downloads.claude.ai/claude-code-releases/<version>/manifest.json,
# platforms.linux-x64.checksum); bump both together. It needs glibc, hence the cc
# runtime below.
ARG CLAUDE_CODE_VERSION=2.1.323
ARG CLAUDE_CODE_SHA256=604d29a6869409106b6a1110bdadcc8b72e8654a5a865d23258f938a696e348e
RUN wget -O /app/claude "https://downloads.claude.ai/claude-code-releases/${CLAUDE_CODE_VERSION}/linux-x64/claude" && \
    echo "${CLAUDE_CODE_SHA256}  /app/claude" | sha256sum -c - && \
    chmod 0755 /app/claude

# Runtime stage: distroless cc (ADR-0092, amended by ADR-0093): no shell or package
# manager, but glibc, which the Claude Code binary links against. Ships CA
# certificates and tzdata. UID 1000 matches runAsUser in the k8s manifests.
# The tools exec kubectl and the bot execs claude directly (no shell), so both only
# need to be on PATH.
FROM gcr.io/distroless/cc-debian13

COPY --from=builder /app/kubectl /usr/local/bin/kubectl
COPY --from=builder /app/claude /usr/local/bin/claude

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
