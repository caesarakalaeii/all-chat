# Build stage
FROM golang:1.27.4-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git

# Set working directory
WORKDIR /build

# Copy go.mod and go.sum from workspace root

# Copy shared module
COPY shared/ ./shared/

# Copy emote-service module
COPY services/emote-service/ ./services/emote-service/

# Set working directory to emote-service
WORKDIR /build/services/emote-service

# Download dependencies
RUN go mod download

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o emote-service ./cmd

# Runtime stage: distroless static (ADR-0092). No shell or package manager;
# ships CA certificates and tzdata. UID 1000 matches runAsUser in the k8s manifests.
FROM gcr.io/distroless/static-debian13

WORKDIR /app

COPY --from=builder /build/services/emote-service/emote-service .
COPY LICENSE ./LICENSE

USER 1000:1000

EXPOSE 8083

CMD ["./emote-service"]
