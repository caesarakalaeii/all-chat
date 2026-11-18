# Build stage
FROM golang:1.27.4-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

WORKDIR /build

# Copy shared module first
COPY shared /build/shared

# Copy modules this service depends on via replace directives
COPY services/message-processor /build/services/message-processor
COPY services/moderation-service /build/services/moderation-service

# Copy service files
COPY services/alert-processor /build/services/alert-processor

WORKDIR /build/services/alert-processor

# Download dependencies
RUN go mod download

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o alert-processor ./cmd

# Runtime stage: distroless static (ADR-0092). No shell or package manager;
# ships CA certificates and tzdata. UID 1000 matches runAsUser in the k8s manifests.
FROM gcr.io/distroless/static-debian13

WORKDIR /app

COPY --from=builder /build/services/alert-processor/alert-processor .
COPY LICENSE ./LICENSE

USER 1000:1000

EXPOSE 8098

ENV PORT=8098
ENV LOG_LEVEL=info

CMD ["./alert-processor"]
