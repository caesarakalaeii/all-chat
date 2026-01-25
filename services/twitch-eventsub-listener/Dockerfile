# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum* ./
COPY shared/ ./shared/

# Copy service files
COPY services/twitch-eventsub-listener/ ./services/twitch-eventsub-listener/

# Download dependencies
WORKDIR /app/services/twitch-eventsub-listener
RUN go mod download

# Build the service
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /app/bin/twitch-eventsub-listener ./cmd

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

# Copy binary from builder
COPY --from=builder /app/bin/twitch-eventsub-listener .

# Expose port
EXPOSE 8090

# Run the service
CMD ["./twitch-eventsub-listener"]
