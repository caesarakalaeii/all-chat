# Build stage
FROM golang:1.26.4-alpine AS builder

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

# Final stage
FROM alpine:3.23

RUN apk --no-cache add ca-certificates && \
    addgroup -g 1000 appuser && adduser -D -u 1000 -G appuser appuser

WORKDIR /app

COPY --from=builder /build/services/twitch-eventsub-listener/twitch-eventsub-listener .
COPY LICENSE ./LICENSE

RUN chown -R appuser:appuser /app

USER appuser

EXPOSE 8090

ENV PORT=8090
ENV LOG_LEVEL=info

CMD ["./twitch-eventsub-listener"]
