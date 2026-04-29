# Build stage
FROM golang:1.26.1-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

WORKDIR /build

# Copy shared module first
COPY shared /build/shared

# Copy service files
COPY services/message-processor /build/services/message-processor

WORKDIR /build/services/message-processor

# Download dependencies
RUN go mod download

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o message-processor ./cmd

# Final stage
FROM alpine:3.21

RUN apk --no-cache add ca-certificates && \
    addgroup -g 1000 appuser && adduser -D -u 1000 -G appuser appuser

WORKDIR /app

COPY --from=builder /build/services/message-processor/message-processor .
COPY LICENSE ./LICENSE

RUN chown -R appuser:appuser /app

USER appuser

EXPOSE 8087

ENV PORT=8087
ENV LOG_LEVEL=info

CMD ["./message-processor"]
