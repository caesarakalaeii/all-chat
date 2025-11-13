# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy go workspace files
COPY go.work go.work.sum ./

# Copy shared module
COPY shared/ ./shared/

# Copy service files
COPY services/auth-service/ ./services/auth-service/

# Download dependencies
WORKDIR /app/services/auth-service
RUN go mod download

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/auth-service ./cmd

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/auth-service .

EXPOSE 8081

CMD ["./auth-service"]
