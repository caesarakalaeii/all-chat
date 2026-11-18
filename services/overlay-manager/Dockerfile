# Build stage
FROM golang:1.27.4-alpine AS builder

WORKDIR /app

# Copy shared module
COPY shared/ ./shared/

# Copy service files
COPY services/overlay-manager/ ./services/overlay-manager/

# Download dependencies
WORKDIR /app/services/overlay-manager
RUN go mod download

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/overlay-manager ./cmd

# Runtime stage: distroless static (ADR-0092). No shell or package manager;
# ships CA certificates and tzdata. UID 1000 matches runAsUser in the k8s manifests.
FROM gcr.io/distroless/static-debian13

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/overlay-manager .
COPY LICENSE ./LICENSE

# Switch to non-root user
USER 1000:1000

# Expose port
EXPOSE 8082

# Run the binary
CMD ["./overlay-manager"]
