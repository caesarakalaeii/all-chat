# Build stage
FROM golang:1.26.3-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

WORKDIR /build

# Copy shared module first
COPY shared /build/shared

# Copy message-processor module (needed for registry import)
COPY services/message-processor /build/services/message-processor

# Copy service files
COPY services/twitch-listener /build/services/twitch-listener

WORKDIR /build/services/twitch-listener

# Download dependencies
RUN go mod download

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o twitch-listener ./cmd

# Final stage
FROM alpine:3.23

RUN apk --no-cache add ca-certificates && \
    addgroup -g 1000 appuser && adduser -D -u 1000 -G appuser appuser

WORKDIR /app

COPY --from=builder /build/services/twitch-listener/twitch-listener .
COPY LICENSE ./LICENSE

RUN chown -R appuser:appuser /app

USER appuser

EXPOSE 8085

ENV PORT=8085
ENV LOG_LEVEL=info

CMD ["./twitch-listener"]
