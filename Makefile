.PHONY: help build test clean docker-up docker-down migrate deps

# Default target
help:
	@echo "All-Chat Makefile Commands"
	@echo ""
	@echo "Development:"
	@echo "  make deps          - Download Go dependencies"
	@echo "  make build         - Build all services"
	@echo "  make test          - Run all tests"
	@echo "  make test-coverage - Run tests with coverage"
	@echo ""
	@echo "Docker Compose:"
	@echo "  make docker-up     - Start all services with Docker Compose"
	@echo "  make docker-down   - Stop all services"
	@echo "  make docker-logs   - View logs"
	@echo "  make docker-restart - Restart all services"
	@echo ""
	@echo "Database:"
	@echo "  make migrate       - Run database migrations"
	@echo "  make migrate-down  - Rollback migrations"
	@echo ""
	@echo "Individual Services:"
	@echo "  make build-auth          - Build auth-service"
	@echo "  make build-overlay       - Build overlay-manager"
	@echo "  make build-emote         - Build emote-service"
	@echo "  make build-gateway       - Build api-gateway"
	@echo "  make build-twitch        - Build twitch-listener"
	@echo "  make build-youtube       - Build youtube-listener"
	@echo "  make build-processor     - Build message-processor"
	@echo "  make build-source-manager - Build source-manager"

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	cd shared && go mod download
	cd services/auth-service && go mod download
	cd services/overlay-manager && go mod download
	cd services/emote-service && go mod download
	cd services/api-gateway && go mod download
	cd services/twitch-listener && go mod download
	cd services/youtube-listener && go mod download
	cd services/message-processor && go mod download
	cd services/source-manager && go mod download

# Build all services
build:
	@echo "Building all services..."
	@$(MAKE) build-auth
	@$(MAKE) build-overlay
	@$(MAKE) build-emote
	@$(MAKE) build-gateway
	@$(MAKE) build-twitch
	@$(MAKE) build-youtube
	@$(MAKE) build-processor
	@$(MAKE) build-source-manager

build-auth:
	@echo "Building auth-service..."
	cd services/auth-service && go build -o ../../bin/auth-service ./cmd

build-overlay:
	@echo "Building overlay-manager..."
	cd services/overlay-manager && go build -o ../../bin/overlay-manager ./cmd

build-emote:
	@echo "Building emote-service..."
	cd services/emote-service && go build -o ../../bin/emote-service ./cmd

build-gateway:
	@echo "Building api-gateway..."
	cd services/api-gateway && go build -o ../../bin/api-gateway ./cmd

build-twitch:
	@echo "Building twitch-listener..."
	cd services/twitch-listener && go build -o ../../bin/twitch-listener ./cmd

build-youtube:
	@echo "Building youtube-listener..."
	cd services/youtube-listener && go build -o ../../bin/youtube-listener ./cmd

build-processor:
	@echo "Building message-processor..."
	cd services/message-processor && go build -o ../../bin/message-processor ./cmd

build-source-manager:
	@echo "Building source-manager..."
	cd services/source-manager && go build -o ../../bin/source-manager ./cmd

# Run tests
test:
	@echo "Running all tests..."
	go test -v ./...

test-coverage:
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

test-auth:
	@echo "Testing auth-service..."
	cd services/auth-service && go test -v ./...

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	rm -f coverage.out coverage.html

# Docker Compose commands
docker-up:
	@echo "Starting services with Docker Compose..."
	cd deployments && docker-compose up -d
	@echo "Services started. Check logs with: make docker-logs"

docker-down:
	@echo "Stopping services..."
	cd deployments && docker-compose down

docker-logs:
	@echo "Tailing logs..."
	cd deployments && docker-compose logs -f

docker-restart:
	@echo "Restarting services..."
	cd deployments && docker-compose restart

docker-build:
	@echo "Building Docker images..."
	cd deployments && docker-compose build

# Database migrations
migrate:
	@echo "Running database migrations..."
	PGPASSWORD=allchat_dev_password psql -h localhost -U allchat -d allchat -f migrations/001_initial_schema.sql

migrate-down:
	@echo "Rolling back migrations..."
	@echo "Manual rollback required - DROP tables in reverse order"

# Run services locally (requires PostgreSQL and Redis running)
run-auth:
	@echo "Running auth-service locally..."
	cd services/auth-service && go run ./cmd
