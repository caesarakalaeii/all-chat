# Build stage
FROM golang:1.27.4-alpine AS builder

WORKDIR /app

# Copy shared module
COPY shared/ ./shared/

# Copy service files
COPY services/api-gateway/ ./services/api-gateway/

# Download dependencies
WORKDIR /app/services/api-gateway
RUN go mod download

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/api-gateway ./cmd

# Runtime stage: distroless static (ADR-0092). No shell or package manager;
# ships CA certificates and tzdata. UID 1000 matches runAsUser in the k8s manifests.
FROM gcr.io/distroless/static-debian13

WORKDIR /app

COPY --from=builder /app/api-gateway .
COPY LICENSE ./LICENSE
COPY static/ ./static/

USER 1000:1000

EXPOSE 8080

ENV PORT=8080
ENV LOG_LEVEL=info

CMD ["./api-gateway"]
