# Build stage
FROM golang:1.26.3-alpine AS builder

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

# Final stage
FROM alpine:3.21

RUN apk --no-cache add ca-certificates && \
    addgroup -g 1000 appuser && adduser -D -u 1000 -G appuser appuser

WORKDIR /app

COPY --from=builder /app/api-gateway .
COPY LICENSE ./LICENSE
COPY static/ ./static/

RUN chown -R appuser:appuser /app

USER appuser

EXPOSE 8080

ENV PORT=8080
ENV LOG_LEVEL=info

CMD ["./api-gateway"]
