.PHONY: help build run test clean docker-up docker-down docker-logs

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Build all services
	@echo "Building all services..."
	@mkdir -p bin
	@go build -o bin/api-gateway ./cmd/api-gateway
	@go build -o bin/auth-service ./cmd/auth-service
	@go build -o bin/overlay-manager ./cmd/overlay-manager
	@go build -o bin/emote-service ./cmd/emote-service
	@go build -o bin/chat-listener ./cmd/chat-listener
	@echo "✓ All services built successfully"

build-api-gateway: ## Build API Gateway
	@mkdir -p bin
	@go build -o bin/api-gateway ./cmd/api-gateway

build-auth: ## Build Auth Service
	@mkdir -p bin
	@go build -o bin/auth-service ./cmd/auth-service

build-overlay: ## Build Overlay Manager
	@mkdir -p bin
	@go build -o bin/overlay-manager ./cmd/overlay-manager

build-emote: ## Build Emote Service
	@mkdir -p bin
	@go build -o bin/emote-service ./cmd/emote-service

build-chat: ## Build Chat Listener
	@mkdir -p bin
	@go build -o bin/chat-listener ./cmd/chat-listener

test: ## Run tests
	@go test -v ./...

test-coverage: ## Run tests with coverage
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out

clean: ## Clean build artifacts
	@rm -rf bin/
	@rm -f coverage.out
	@echo "✓ Cleaned build artifacts"

docker-up: ## Start all services with Docker Compose
	@cd deployments && docker-compose up -d
	@echo "✓ All services started"

docker-down: ## Stop all services
	@cd deployments && docker-compose down
	@echo "✓ All services stopped"

docker-logs: ## Show logs from all services
	@cd deployments && docker-compose logs -f

docker-build: ## Build Docker images
	@cd deployments && docker-compose build

docker-restart: docker-down docker-up ## Restart all services

deps: ## Download Go dependencies
	@go mod download
	@go mod tidy

lint: ## Run linter
	@golangci-lint run ./...

fmt: ## Format code
	@go fmt ./...

run-auth: ## Run auth service locally
	@go run ./cmd/auth-service/main.go

run-overlay: ## Run overlay manager locally
	@go run ./cmd/overlay-manager/main.go

run-emote: ## Run emote service locally
	@go run ./cmd/emote-service/main.go

run-chat: ## Run chat listener locally
	@go run ./cmd/chat-listener/main.go

run-gateway: ## Run API gateway locally
	@go run ./cmd/api-gateway/main.go

migrate-up: ## Run database migrations
	@psql postgresql://allchat:allchat_dev_password@localhost:5432/allchat < migrations/001_initial_schema.sql

web-install: ## Install frontend dependencies
	@cd web && npm install

web-dev: ## Start frontend dev server
	@cd web && npm run dev

web-build: ## Build frontend for production
	@cd web && npm run build

.DEFAULT_GOAL := help
