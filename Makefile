VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_HEAD := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

BUILD_PATH := build
BIN_PATH := bin
BUILD_FLAGS := -mod=readonly -v
LD_FLAGS := -X main.Version=$(VERSION) -X main.GitHead=$(GIT_HEAD)

UNAME_S := $(shell uname -s | tr A-Z a-z)
UNAME_M := $(shell uname -m)

TOOLS_DIR := $(shell pwd)/tools
LOCAL_BIN := $(shell pwd)/.bin
export PATH := $(LOCAL_BIN):$(PATH)

GREEN  := $(shell tput -Txterm setaf 2)
YELLOW := $(shell tput -Txterm setaf 3)
CYAN   := $(shell tput -Txterm setaf 6)
RESET  := $(shell tput -Txterm sgr0)

# Build functions
define fn_build
@echo "$(YELLOW)Building analyzer for $(UNAME_S)/$(UNAME_M)$(RESET)"
@mkdir -p $(BIN_PATH)
CGO_ENABLED=0 go build $(BUILD_FLAGS) -ldflags="$(LD_FLAGS)" -o $(BIN_PATH)/analyzer ./cmd
@echo "$(GREEN)✓ Binary built: $(BIN_PATH)/analyzer$(RESET)"
endef

define fn_build_linux
@echo "$(YELLOW)Building analyzer for linux/amd64$(RESET)"
@mkdir -p $(BUILD_PATH)/linux/amd64
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(BUILD_FLAGS) -ldflags="$(LD_FLAGS)" -o $(BUILD_PATH)/linux/amd64/analyzer ./cmd
@echo "$(GREEN)✓ linux/amd64 complete$(RESET)"

@echo "$(YELLOW)Building analyzer for linux/arm64$(RESET)"
@mkdir -p $(BUILD_PATH)/linux/arm64
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build $(BUILD_FLAGS) -ldflags="$(LD_FLAGS)" -o $(BUILD_PATH)/linux/arm64/analyzer ./cmd
@echo "$(GREEN)✓ linux/arm64 complete$(RESET)"
endef

.PHONY: help
help: ## Show this help
	@echo ''
	@echo 'Usage:'
	@echo '  ${YELLOW}make${RESET} ${GREEN}<target>${RESET}'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} { \
		if (/^[a-zA-Z_.-]+:.*?##.*$$/) {printf "    ${YELLOW}%-20s${GREEN}%s${RESET}\n", $$1, $$2} \
		else if (/^## .*$$/) {printf "  ${CYAN}%s${RESET}\n", substr($$1,4)} \
		}' $(MAKEFILE_LIST)

## General:

download:
	@echo "$(YELLOW)Downloading go.mod dependencies$(RESET)"
	@go mod download

.PHONY: install
install: download ## Install dev tools
	@echo "$(YELLOW)Installing tools from $(TOOLS_DIR)/tools.go$(RESET)"
	@mkdir -p $(LOCAL_BIN)
	@cd $(TOOLS_DIR) && cat tools.go | grep _ | awk -F'"' '{print $$2}' | GOBIN=$(LOCAL_BIN) xargs -I % go install %
	@echo "$(GREEN)✓ Tools installed$(RESET)"

## Development:

.PHONY: clean
clean: ## Clean build artifacts
	@rm -rf $(BIN_PATH)/ $(BUILD_PATH)/
	@rm -f coverage.out coverage.html

.PHONY: lint
lint: ## Lint code
	@$(LOCAL_BIN)/golangci-lint run ./...

.PHONY: test
test: ## Run tests
	@go test ./... -v -race -count=1 -coverprofile=coverage.out

.PHONY: cover
cover: test ## Test and show coverage
	@go tool cover -html=coverage.out

## Build:

.PHONY: build
build: ## Build binary
	$(call fn_build)

.PHONY: build.linux
build.linux: ## Build for Linux (amd64 + arm64)
	$(call fn_build_linux)

## Docker:

.PHONY: docker
docker: build.linux ## Build Docker image
	@docker build -t page-analyzer:latest .

.PHONY: docker-up
docker-up: ## Start services
	@docker-compose up -d

.PHONY: docker-down
docker-down: ## Stop services
	@docker-compose down

.PHONY: docker-logs
docker-logs: ## Follow logs
	@docker-compose logs -f analyzer
