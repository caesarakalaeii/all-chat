# Build stage
FROM golang:1.27.4-alpine AS builder

WORKDIR /app

# Copy modules this service depends on via replace directives
COPY shared/ ./shared/
COPY services/message-processor/ ./services/message-processor/

# Copy service files
COPY services/engagement-service/ ./services/engagement-service/

# Download dependencies
WORKDIR /app/services/engagement-service
RUN go mod download

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/engagement-service ./cmd

# Runtime stage: distroless static (ADR-0092). No shell or package manager;
# ships CA certificates and tzdata. UID 1000 matches runAsUser in the k8s manifests.
FROM gcr.io/distroless/static-debian13

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/engagement-service .
COPY LICENSE ./LICENSE

# Switch to non-root user
USER 1000:1000

# Expose port
EXPOSE 8093

# Run the binary
CMD ["./engagement-service"]
