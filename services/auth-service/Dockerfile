# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy shared module
COPY shared/ ./shared/

# Copy service files
COPY services/auth-service/ ./services/auth-service/

# Download dependencies (use go.mod directly, not go.work)
WORKDIR /app/services/auth-service
RUN go work vendor || go mod download

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/auth-service ./cmd

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/auth-service .

EXPOSE 8081

CMD ["./auth-service"]
