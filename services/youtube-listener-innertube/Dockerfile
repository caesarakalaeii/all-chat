# Build stage
FROM golang:1.27.4-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

WORKDIR /build

# Copy service files (build from repo root, not service directory)
COPY . /build

WORKDIR /build/services/youtube-listener-innertube

# Download dependencies
RUN go mod download

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o youtube-listener-innertube ./cmd

# Runtime stage: distroless static (ADR-0092). No shell or package manager;
# ships CA certificates and tzdata. UID 1000 matches runAsUser in the k8s manifests.
FROM gcr.io/distroless/static-debian13

WORKDIR /app

COPY --from=builder /build/services/youtube-listener-innertube/youtube-listener-innertube .
COPY LICENSE ./LICENSE

USER 1000:1000

EXPOSE 8080

ENV PORT=8080
ENV LOG_LEVEL=info

CMD ["./youtube-listener-innertube"]
