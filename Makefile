.PHONY: help build build-all test clean docker-up docker-down migrate deps frontend-dev test-stream test-stream-stop

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
	@echo "Frontend Development:"
	@echo "  make frontend-quick      - Quick start (all-in-one: start + seed + verify)"
	@echo "  make frontend-dev        - Start minimal backend for frontend dev"
	@echo "  make frontend-down       - Stop frontend dev environment"
	@echo "  make frontend-seed       - Seed test data"
	@echo "  make frontend-messages   - Generate test messages"
	@echo "  make frontend-verify     - Verify frontend setup is working"
	@echo "  make frontend-reset      - Reset frontend dev environment"
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
	@echo "  make build-twitch-eventsub - Build twitch-eventsub-listener"
	@echo "  make build-youtube       - Build youtube-listener"
	@echo "  make build-tiktok        - Build tiktok-listener (Node.js)"
	@echo "  make build-processor     - Build message-processor"
	@echo "  make build-source-manager - Build source-manager"
	@echo "  make build-token-refresh - Build token-refresh-service"
	@echo ""
	@echo "CI Targets:"
	@echo "  make build-all     - Build all Go listener modules (CI target)"

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	cd shared && go mod download
	cd services/auth-service && go mod download
	cd services/overlay-manager && go mod download
	cd services/emote-service && go mod download
	cd services/api-gateway && go mod download
	cd services/twitch-listener && go mod download
	cd services/twitch-eventsub-listener && go mod download
	cd services/youtube-listener && go mod download
	cd services/tiktok-listener && npm install
	cd services/message-processor && go mod download
	cd services/source-manager && go mod download
	cd services/token-refresh-service && go mod download

# Build all Go listener modules — verifies replace-directive version consistency across all modules
# Note: tiktok-listener is Node.js and is intentionally excluded
build-all:
	@echo "Building all Go listener modules..."
	cd /home/moersener/Hobby/all-chat/shared && go build ./...
	cd /home/moersener/Hobby/all-chat/services/twitch-listener && go build ./...
	cd /home/moersener/Hobby/all-chat/services/kick-listener && go build ./...
	cd /home/moersener/Hobby/all-chat/services/twitch-eventsub-listener && go build ./...
	cd /home/moersener/Hobby/all-chat/services/youtube-listener && go build ./...
	cd /home/moersener/Hobby/all-chat/services/youtube-listener-innertube && go build ./...
	cd /home/moersener/Hobby/all-chat/services/discord-listener && go build ./...
	@echo "All listener modules built successfully"

# Build all services
build:
	@echo "Building all services..."
	@$(MAKE) build-auth
	@$(MAKE) build-overlay
	@$(MAKE) build-emote
	@$(MAKE) build-gateway
	@$(MAKE) build-twitch
	@$(MAKE) build-twitch-eventsub
	@$(MAKE) build-youtube
	@$(MAKE) build-tiktok
	@$(MAKE) build-processor
	@$(MAKE) build-source-manager
	@$(MAKE) build-token-refresh

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

build-twitch-eventsub:
	@echo "Building twitch-eventsub-listener..."
	cd services/twitch-eventsub-listener && go build -o ../../bin/twitch-eventsub-listener ./cmd

build-youtube:
	@echo "Building youtube-listener..."
	cd services/youtube-listener && go build -o ../../bin/youtube-listener ./cmd

build-tiktok:
	@echo "Building tiktok-listener..."
	cd services/tiktok-listener && npm install && npm run build

build-processor:
	@echo "Building message-processor..."
	cd services/message-processor && go build -o ../../bin/message-processor ./cmd

build-source-manager:
	@echo "Building source-manager..."
	cd services/source-manager && go build -o ../../bin/source-manager ./cmd

build-token-refresh:
	@echo "Building token-refresh-service..."
	cd services/token-refresh-service && go build -o ../../bin/token-refresh-service ./cmd

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

# Frontend development environment
frontend-quick:
	@echo "Running complete frontend setup..."
	@./scripts/quick-start-frontend.sh

frontend-dev:
	@echo "Starting minimal backend for frontend development..."
	docker-compose -f docker-compose.frontend.yml up -d
	@echo ""
	@echo "Waiting for services to be healthy..."
	@sleep 10
	@echo ""
	@echo "Services started! Next steps:"
	@echo "  1. Seed test data: make frontend-seed"
	@echo "  2. Generate messages: make frontend-messages"
	@echo "  3. Start frontend: cd frontend && npm run dev"
	@echo ""
	@echo "View logs: docker-compose -f docker-compose.frontend.yml logs -f"

frontend-down:
	@echo "Stopping frontend dev environment..."
	docker-compose -f docker-compose.frontend.yml down

frontend-seed:
	@echo "Seeding test data..."
	@./scripts/seed-test-data.sh

frontend-messages:
	@echo "Starting message generator..."
	@echo "Press Ctrl+C to stop"
	@./scripts/generate-test-messages.sh

test-stream:
	@echo "Starting public test-stream generator..."
	@./scripts/start-test-stream.sh start

test-stream-stop:
	@echo "Stopping public test-stream generator..."
	@./scripts/start-test-stream.sh stop

frontend-verify:
	@echo "Verifying frontend development environment..."
	@./scripts/verify-frontend-setup.sh

frontend-reset:
	@echo "Resetting frontend dev environment..."
	docker-compose -f docker-compose.frontend.yml down -v
	@echo "Starting fresh..."
	@$(MAKE) frontend-dev
	@sleep 10
	@$(MAKE) frontend-seed
	@echo ""
	@echo "Environment reset complete!"
