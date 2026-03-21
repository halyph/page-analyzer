.PHONY: help build run test docker-build docker-up docker-down clean

# Default target
.DEFAULT_GOAL := help

# Colors for output
CYAN := \033[36m
RESET := \033[0m

help: ## Show this help message
	@echo "$(CYAN)Page Analyzer - Available Commands$(RESET)"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(CYAN)%-20s$(RESET) %s\n", $$1, $$2}'
	@echo ""

# Build targets
build: ## Build the binary
	@echo "$(CYAN)Building binary...$(RESET)"
	go build -o bin/analyzer ./cmd
	@echo "✅ Binary built: bin/analyzer"

build-all: ## Build for multiple platforms
	@echo "$(CYAN)Building for multiple platforms...$(RESET)"
	GOOS=linux GOARCH=amd64 go build -o bin/analyzer-linux-amd64 ./cmd
	GOOS=darwin GOARCH=amd64 go build -o bin/analyzer-darwin-amd64 ./cmd
	GOOS=darwin GOARCH=arm64 go build -o bin/analyzer-darwin-arm64 ./cmd
	GOOS=windows GOARCH=amd64 go build -o bin/analyzer-windows-amd64.exe ./cmd
	@echo "✅ All binaries built"

# Run targets
run: ## Run the service locally
	@echo "$(CYAN)Starting analyzer service...$(RESET)"
	go run ./cmd serve

run-cli: ## Run CLI analyzer (usage: make run-cli URL=https://example.com)
	@echo "$(CYAN)Analyzing $(URL)...$(RESET)"
	go run ./cmd analyze $(URL)

run-cli-json: ## Run CLI analyzer with JSON output
	@echo "$(CYAN)Analyzing $(URL) (JSON output)...$(RESET)"
	go run ./cmd analyze $(URL) --json

# Test targets
test: ## Run all tests
	@echo "$(CYAN)Running tests...$(RESET)"
	go test ./... -v -race -coverprofile=coverage.out

test-unit: ## Run unit tests only
	@echo "$(CYAN)Running unit tests...$(RESET)"
	go test ./internal/... -v -short

test-coverage: test ## Run tests and show coverage report
	@echo "$(CYAN)Generating coverage report...$(RESET)"
	go tool cover -html=coverage.out -o coverage.html
	@echo "✅ Coverage report: coverage.html"

test-integration: ## Run integration tests
	@echo "$(CYAN)Running integration tests...$(RESET)"
	go test ./test/integration/... -v

test-watch: ## Run tests in watch mode (requires entr)
	find . -name '*.go' | entr -c go test ./...

# Code quality
lint: ## Run linters
	@echo "$(CYAN)Running linters...$(RESET)"
	golangci-lint run

fmt: ## Format code
	@echo "$(CYAN)Formatting code...$(RESET)"
	go fmt ./...
	gofmt -s -w .

vet: ## Run go vet
	@echo "$(CYAN)Running go vet...$(RESET)"
	go vet ./...

# Docker targets
docker-build: ## Build Docker image
	@echo "$(CYAN)Building Docker image...$(RESET)"
	docker build -t page-analyzer:latest .
	@echo "✅ Image built: page-analyzer:latest"

docker-up: ## Start all services with docker-compose
	@echo "$(CYAN)Starting services...$(RESET)"
	docker-compose up -d
	@echo "✅ Services started"
	@echo ""
	@echo "Access points:"
	@echo "  App:        http://localhost:8080"
	@echo "  Grafana:    http://localhost:3000 (admin/admin)"
	@echo "  Prometheus: http://localhost:9090"

docker-logs: ## Follow logs
	docker-compose logs -f analyzer

docker-down: ## Stop all services
	@echo "$(CYAN)Stopping services...$(RESET)"
	docker-compose down

docker-clean: ## Stop services and remove volumes
	@echo "$(CYAN)Cleaning up...$(RESET)"
	docker-compose down -v
	@echo "✅ Cleanup complete"

docker-restart: docker-down docker-up ## Restart all services

# Development targets
dev: ## Start infrastructure only (run app with 'make run')
	@echo "$(CYAN)Starting development infrastructure...$(RESET)"
	docker-compose up -d redis tempo prometheus grafana
	@echo "✅ Infrastructure ready"
	@echo ""
	@echo "Next steps:"
	@echo "  1. Run the app:  make run"
	@echo "  2. Visit:        http://localhost:8080"
	@echo "  3. Grafana:      http://localhost:3000 (admin/admin)"

dev-down: ## Stop development infrastructure
	docker-compose stop redis tempo prometheus grafana

# Demo targets
demo: ## Run demo script
	@echo "$(CYAN)Running demo...$(RESET)"
	./scripts/demo.sh

load-test: ## Run load test (requires hey)
	@echo "$(CYAN)Running load test...$(RESET)"
	@command -v hey >/dev/null 2>&1 || { echo "hey not installed. Run: go install github.com/rakyll/hey@latest"; exit 1; }
	hey -n 100 -c 10 -m POST \
		-H "Content-Type: application/json" \
		-d '{"url":"https://example.com"}' \
		http://localhost:8080/api/analyze

# Cleanup
clean: ## Clean build artifacts and caches
	@echo "$(CYAN)Cleaning...$(RESET)"
	rm -rf bin/
	rm -f coverage.out coverage.html
	go clean -cache -testcache
	@echo "✅ Clean complete"

clean-all: clean docker-clean ## Deep clean (includes Docker volumes)

# Dependencies
deps: ## Download dependencies
	@echo "$(CYAN)Downloading dependencies...$(RESET)"
	go mod download

deps-update: ## Update dependencies
	@echo "$(CYAN)Updating dependencies...$(RESET)"
	go get -u ./...
	go mod tidy

deps-tidy: ## Tidy dependencies
	@echo "$(CYAN)Tidying dependencies...$(RESET)"
	go mod tidy

# Installation
install-tools: ## Install development tools
	@echo "$(CYAN)Installing development tools...$(RESET)"
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/rakyll/hey@latest
	go install github.com/cosmtrek/air@latest
	@echo "✅ Tools installed"

# Git hooks
git-hooks: ## Install git hooks
	@echo "$(CYAN)Installing git hooks...$(RESET)"
	@echo "#!/bin/bash\nmake fmt && make test" > .git/hooks/pre-commit
	@chmod +x .git/hooks/pre-commit
	@echo "✅ Git hooks installed"

# Info
info: ## Show project information
	@echo "$(CYAN)Project Information$(RESET)"
	@echo "  Name:    page-analyzer"
	@echo "  Go:      $$(go version)"
	@echo "  Module:  $$(go list -m)"
	@echo ""
	@echo "$(CYAN)Directory Structure:$(RESET)"
	@tree -d -L 2 -I 'bin|tmp|vendor'
