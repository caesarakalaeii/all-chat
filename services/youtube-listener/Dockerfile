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
COPY services/youtube-listener /build/services/youtube-listener

WORKDIR /build/services/youtube-listener

# Download dependencies
RUN go mod download

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o youtube-listener ./cmd

# Final stage
FROM alpine:3.21

RUN apk --no-cache add ca-certificates tzdata && \
    addgroup -g 1000 appuser && adduser -D -u 1000 -G appuser appuser

WORKDIR /app

COPY --from=builder /build/services/youtube-listener/youtube-listener .
COPY LICENSE ./LICENSE

RUN chown -R appuser:appuser /app

USER appuser

EXPOSE 8086

ENV PORT=8086
ENV LOG_LEVEL=info

CMD ["./youtube-listener"]
