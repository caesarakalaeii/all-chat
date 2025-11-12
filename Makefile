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
	@echo "  make build-auth    - Build auth-service"
	@echo "  make test-auth     - Test auth-service"
	@echo "  make run-auth      - Run auth-service locally"

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	cd shared && go mod download
	cd services/auth-service && go mod download

# Build all services
build:
	@echo "Building all services..."
	@$(MAKE) build-auth

build-auth:
	@echo "Building auth-service..."
	cd services/auth-service && go build -o ../../bin/auth-service ./cmd

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
