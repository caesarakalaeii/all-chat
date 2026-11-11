# Build stage
FROM golang:1.27.4-alpine AS builder

WORKDIR /app

# Copy modules this service depends on via replace directives
COPY shared/ ./shared/

# Copy service files
COPY services/media-service/ ./services/media-service/

# Download dependencies
WORKDIR /app/services/media-service
RUN go mod download

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/media-service ./cmd

# Runtime stage: distroless static (ADR-0092). No shell or package manager;
# ships CA certificates and tzdata. UID 1000 matches runAsUser in the k8s manifests.
FROM gcr.io/distroless/static-debian13

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/media-service .
COPY LICENSE ./LICENSE

# Switch to non-root user
USER 1000:1000

# Expose port
EXPOSE 8094

# Run the binary
CMD ["./media-service"]
