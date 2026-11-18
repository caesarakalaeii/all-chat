# Build stage
FROM golang:1.27.4-alpine AS builder

WORKDIR /app

# Copy modules this service depends on via replace directives
COPY shared/ ./shared/

# Copy service files
COPY services/youtube-quota-monitor/ ./services/youtube-quota-monitor/

# Download dependencies
WORKDIR /app/services/youtube-quota-monitor
RUN go mod download

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/youtube-quota-monitor ./cmd

# Runtime stage: distroless static (ADR-0092). No shell or package manager;
# ships CA certificates and tzdata. UID 1000 matches runAsUser in the k8s manifests.
FROM gcr.io/distroless/static-debian13

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/youtube-quota-monitor .
COPY LICENSE ./LICENSE

# Switch to non-root user
USER 1000:1000

# Expose port
EXPOSE 8093

# Run the binary
CMD ["./youtube-quota-monitor"]
