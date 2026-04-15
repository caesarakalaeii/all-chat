# Build stage
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

WORKDIR /build

# Copy service files (build from repo root, not service directory)
COPY . /build

WORKDIR /build/services/youtube-listener-innertube

# Download dependencies
RUN go mod download

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o youtube-listener-innertube ./cmd

# Final stage
FROM alpine:3.21

RUN apk --no-cache add ca-certificates && \
    addgroup -g 1000 appuser && adduser -D -u 1000 -G appuser appuser

WORKDIR /app

COPY --from=builder /build/services/youtube-listener-innertube/youtube-listener-innertube .
COPY LICENSE ./LICENSE

RUN chown -R appuser:appuser /app

USER appuser

EXPOSE 8080

ENV PORT=8080
ENV LOG_LEVEL=info

CMD ["./youtube-listener-innertube"]
