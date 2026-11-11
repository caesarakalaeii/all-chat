# Build stage
FROM golang:1.27.4-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

WORKDIR /build

# Copy shared module first
COPY shared /build/shared

# Copy the message-processor module — the listener imports its message-ID registry
# (deletion handling, ADR-0015) via a local replace directive, so its source must be in
# the build context for `go mod download`/build to resolve it.
COPY services/message-processor /build/services/message-processor

# Copy service files
COPY services/twitch-eventsub-listener /build/services/twitch-eventsub-listener

WORKDIR /build/services/twitch-eventsub-listener

# Download dependencies
RUN go mod download

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o twitch-eventsub-listener ./cmd

# Runtime stage: distroless static (ADR-0092). No shell or package manager;
# ships CA certificates and tzdata. UID 1000 matches runAsUser in the k8s manifests.
FROM gcr.io/distroless/static-debian13

WORKDIR /app

COPY --from=builder /build/services/twitch-eventsub-listener/twitch-eventsub-listener .
COPY LICENSE ./LICENSE

USER 1000:1000

EXPOSE 8090

ENV PORT=8090
ENV LOG_LEVEL=info

CMD ["./twitch-eventsub-listener"]
