# Build stage
FROM golang:1.27.1-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

WORKDIR /build

# Copy shared module first
COPY shared /build/shared

# Copy modules this service depends on via replace directives
COPY services/message-processor /build/services/message-processor

# Copy service files
COPY services/alert-processor /build/services/alert-processor

WORKDIR /build/services/alert-processor

# Download dependencies
RUN go mod download

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o alert-processor ./cmd

# Final stage
FROM alpine:3.24

RUN apk --no-cache add ca-certificates && \
    addgroup -g 1000 appuser && adduser -D -u 1000 -G appuser appuser

WORKDIR /app

COPY --from=builder /build/services/alert-processor/alert-processor .
COPY LICENSE ./LICENSE

RUN chown -R appuser:appuser /app

USER appuser

EXPOSE 8095

ENV PORT=8095
ENV LOG_LEVEL=info

CMD ["./alert-processor"]
