# Build stage
FROM golang:1.27.4-alpine AS builder

WORKDIR /app

# Copy shared module
COPY shared/ ./shared/

# Copy service files
COPY services/picarto-listener/ ./services/picarto-listener/

# Download dependencies
WORKDIR /app/services/picarto-listener
RUN go mod download

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/picarto-listener ./cmd

# Runtime stage: distroless static (ADR-0092). No shell or package manager;
# ships CA certificates and tzdata. UID 1000 matches runAsUser in the k8s manifests.
FROM gcr.io/distroless/static-debian13

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/picarto-listener .
COPY LICENSE ./LICENSE

USER 1000:1000

# Expose health check port
EXPOSE 8097

CMD ["./picarto-listener"]
