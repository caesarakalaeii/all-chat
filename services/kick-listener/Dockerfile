# Build stage
FROM golang:1.27.4-alpine AS builder

WORKDIR /app

# Copy shared module
COPY shared/ ./shared/

# Copy service files
COPY services/kick-listener/ ./services/kick-listener/

# Download dependencies
WORKDIR /app/services/kick-listener
RUN go mod download

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/kick-listener ./cmd

# Runtime stage: distroless static (ADR-0092). No shell or package manager;
# ships CA certificates and tzdata. UID 1000 matches runAsUser in the k8s manifests.
FROM gcr.io/distroless/static-debian13

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/kick-listener .
COPY LICENSE ./LICENSE

USER 1000:1000

# Expose health check port
EXPOSE 8089

CMD ["./kick-listener"]
