# Build stage
FROM golang:1.27.4-alpine AS builder

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

# Runtime stage: distroless static (ADR-0092). No shell or package manager;
# ships CA certificates and tzdata. UID 1000 matches runAsUser in the k8s manifests.
FROM gcr.io/distroless/static-debian13

WORKDIR /app

COPY --from=builder /build/services/message-processor/message-processor .
COPY LICENSE ./LICENSE

USER 1000:1000

EXPOSE 8087

ENV PORT=8087
ENV LOG_LEVEL=info

CMD ["./message-processor"]
