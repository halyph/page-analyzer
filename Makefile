.PHONY: help build run test docker-build docker-up docker-down clean

# Default target
.DEFAULT_GOAL := help

# Version information
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_HEAD := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Build settings
BUILD_PATH := build
BIN_PATH := bin
BUILD_FLAGS := -mod=readonly -v
LD_FLAGS := -X main.Version=$(VERSION) -X main.GitHead=$(GIT_HEAD)

# Detect OS and architecture
UNAME_S := $(shell uname -s | tr A-Z a-z)
UNAME_M := $(shell uname -m)

# Tool management
TOOLS_DIR := $(shell pwd)/tools
LOCAL_BIN := $(shell pwd)/.bin
export PATH := $(LOCAL_BIN):$(PATH)

# Colors (using tput for better compatibility)
GREEN  := $(shell tput -Txterm setaf 2)
YELLOW := $(shell tput -Txterm setaf 3)
CYAN   := $(shell tput -Txterm setaf 6)
RESET  := $(shell tput -Txterm sgr0)

help: ## Show this help message
	@echo "$(CYAN)Page Analyzer - Available Commands$(RESET)"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(CYAN)%-20s$(RESET) %s\n", $$1, $$2}'
	@echo ""

## Build:

# Build functions
define fn_build
@echo "$(GREEN)Building analyzer for $(UNAME_S)/$(UNAME_M)$(RESET)"
@mkdir -p $(BIN_PATH)
CGO_ENABLED=0 go build $(BUILD_FLAGS) -ldflags="$(LD_FLAGS)" -o $(BIN_PATH)/analyzer ./cmd
@echo "$(GREEN)✓ Binary built: $(BIN_PATH)/analyzer$(RESET)"
endef

define fn_build_linux
@echo "$(GREEN)Building analyzer for linux/amd64$(RESET)"
@mkdir -p $(BUILD_PATH)/linux/amd64
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(BUILD_FLAGS) -ldflags="$(LD_FLAGS)" -o $(BUILD_PATH)/linux/amd64/analyzer ./cmd
@echo "$(GREEN)✓ linux/amd64 build complete$(RESET)"

@echo "$(GREEN)Building analyzer for linux/arm64$(RESET)"
@mkdir -p $(BUILD_PATH)/linux/arm64
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build $(BUILD_FLAGS) -ldflags="$(LD_FLAGS)" -o $(BUILD_PATH)/linux/arm64/analyzer ./cmd
@echo "$(GREEN)✓ linux/arm64 build complete$(RESET)"
endef

.PHONY: build
build: ## Build for current platform
	$(call fn_build)

.PHONY: build.linux
build.linux: ## Build for Linux (amd64 + arm64)
	$(call fn_build_linux)

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

## Testing:

.PHONY: test
test: ## Run all tests
	@echo "$(YELLOW)Running tests...$(RESET)"
	@go test ./... -v -race -count=1 -coverprofile=coverage.out
	@echo "$(GREEN)✓ All tests passed$(RESET)"

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

## Development:

.PHONY: download
download: ## Download dependencies
	@echo "$(YELLOW)Downloading go.mod dependencies$(RESET)"
	@go mod download

.PHONY: install
install: download ## Install dev tools locally
	@echo "$(YELLOW)Installing tools from $(TOOLS_DIR)/tools.go$(RESET)"
	@mkdir -p $(LOCAL_BIN)
	@cd $(TOOLS_DIR) && cat tools.go | grep _ | awk -F'"' '{print $$2}' | GOBIN=$(LOCAL_BIN) xargs -I % go install %
	@echo "$(GREEN)✓ Tools installed to $(LOCAL_BIN)$(RESET)"

## Code Quality:

.PHONY: lint
lint: ## Run linters
	@echo "$(YELLOW)Running golangci-lint...$(RESET)"
	@$(LOCAL_BIN)/golangci-lint run ./...

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

## Cleanup:

.PHONY: clean
clean: ## Clean build artifacts and caches
	@echo "$(YELLOW)Cleaning build artifacts...$(RESET)"
	@rm -rf $(BIN_PATH)/ $(BUILD_PATH)/
	@rm -f coverage.out coverage.html
	@go clean -cache -testcache
	@echo "$(GREEN)✓ Clean complete$(RESET)"

.PHONY: clean-tools
clean-tools: ## Clean installed tools (use with caution)
	@echo "$(YELLOW)Cleaning installed tools...$(RESET)"
	@rm -rf $(LOCAL_BIN)
	@echo "$(GREEN)✓ Tools cleaned$(RESET)"

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
