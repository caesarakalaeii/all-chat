# Build stage
FROM golang:1.27.4-alpine AS builder

WORKDIR /app

# Copy shared module
COPY shared/ ./shared/

# Copy service files
COPY services/share-service/ ./services/share-service/

# Download dependencies
WORKDIR /app/services/share-service
RUN go mod download

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/share-service ./cmd

# Runtime stage: distroless static (ADR-0092). No shell or package manager;
# ships CA certificates and tzdata. UID 1000 matches runAsUser in the k8s manifests.
FROM gcr.io/distroless/static-debian13

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/share-service .
COPY LICENSE ./LICENSE

# Switch to non-root user
USER 1000:1000

# Expose port
EXPOSE 8090

# Run the binary
CMD ["./share-service"]
