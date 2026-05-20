# Build stage
FROM golang:1.26.3-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

WORKDIR /build

# Copy shared module first
COPY shared /build/shared

# Copy service files
COPY services/source-manager /build/services/source-manager

WORKDIR /build/services/source-manager

# Download dependencies
RUN go mod download

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o source-manager ./cmd

# Final stage
FROM alpine:3.23

RUN apk --no-cache add ca-certificates && \
    addgroup -g 1000 appuser && adduser -D -u 1000 -G appuser appuser

WORKDIR /app

COPY --from=builder /build/services/source-manager/source-manager .
COPY LICENSE ./LICENSE

RUN chown -R appuser:appuser /app

USER appuser

EXPOSE 8088

ENV PORT=8088
ENV LOG_LEVEL=info

CMD ["./source-manager"]
