VERSION         ?= latest
TEAM            ?= page-analyzer
APPLICATION     ?= page-analyzer
DOCKER_REGISTRY ?= localhost:5000

BINARIES       := $(shell ls ./cmd)
SOURCES        := $(shell find . -name '*.go' 2>/dev/null | grep -v vendor)
MOCK_PACKAGES  := $(shell find . -name "mocks" -o -name "mock_*" 2>/dev/null | grep -v "vendor")

PACKAGES       := $(shell go list -f '{{.Dir}}' ./...)

BUILD_PATH     := build
COVER_FILE     := $(BUILD_PATH)/coverprofile.txt

GIT_HEAD          := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS           := -X main.Version=$(VERSION) -X main.GitHead=$(GIT_HEAD) -s -w
TEST_FLAGS        := -race -count=1 -mod=readonly -cover -coverprofile $(COVER_FILE)
TEST_FLAGS_INTEG  := $(TEST_FLAGS) -tags=integration -timeout=5m
BUILD_FLAGS       := -mod=readonly -v

# Detect OS and architecture
# linux/darwin and x86_64/arm64
UNAME_S := $(shell uname -s | tr A-Z a-z)
UNAME_M := $(shell uname -m)

## BEGIN of General

GREEN  := $(shell tput -Txterm setaf 2)
YELLOW := $(shell tput -Txterm setaf 3)
WHITE  := $(shell tput -Txterm setaf 7)
CYAN   := $(shell tput -Txterm setaf 6)
RESET  := $(shell tput -Txterm sgr0)

.PHONY: help
help: ## Show this help
	@echo ''
	@echo 'Usage:'
	@echo '  ${YELLOW}make${RESET} ${GREEN}<target>${RESET}'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} { \
		if (/^## BEGIN of/) {section = substr($$0, index($$0, "BEGIN of") + 8); printf "${CYAN}%s${RESET}\n", section} \
		else if  (/^[a-zA-Z0-9._-]+:.*##.*$$/) { \
 			if (section != "") { \
			printf "    ${YELLOW}%-20s${GREEN}%s${RESET}\n", $$1, $$2 \
			} \
		} \
		else if (/^## END/) {section = ""} \
	}' $(MAKEFILE_LIST)

## END of General

## BEGIN of building binaries
# map hardware architecture to GOARCH
ifeq ($(UNAME_M),x86_64)
    GOARCH := amd64
else ifeq ($(UNAME_M),arm64)
    GOARCH := arm64
endif

.PHONY: build
build: build.local ## alias for `build.local` (Build the binary for the local OS and architecture)

.PHONY: build.local
build.local: $(addprefix $(BUILD_PATH)/$(UNAME_S)/$(GOARCH)/,$(BINARIES)) ## Build the binary for the local OS and architecture

.PHONY: build.linux
build.linux: $(addprefix $(BUILD_PATH)/linux/amd64/,$(BINARIES)) $(addprefix $(BUILD_PATH)/linux/arm64/,$(BINARIES)) ## Build the binary for linux OS and architecture

.PHONY: build.darwin
build.darwin: $(addprefix $(BUILD_PATH)/darwin/amd64/,$(BINARIES)) $(addprefix $(BUILD_PATH)/darwin/arm64/,$(BINARIES)) ## Build the binary for darwin OS and architecture

# build go application: $(call fn_build,1:goos,2:goarch,3:binary_name)
define fn_build
GOOS=$(1) GOARCH=$(2) CGO_ENABLED=0 go build $(BUILD_FLAGS) -ldflags="$(LDFLAGS)" -o ./$(BUILD_PATH)/$(1)/$(2)/$(3) ./cmd/$(3)
@echo $(3) build for $(1)/$(2) complete
endef

$(BUILD_PATH)/linux/amd64/%: $(SOURCES)
	$(call fn_build,linux,amd64,$*)

$(BUILD_PATH)/linux/arm64/%: $(SOURCES)
	$(call fn_build,linux,arm64,$*)

$(BUILD_PATH)/darwin/amd64/%: $(SOURCES)
	$(call fn_build,darwin,amd64,$*)

$(BUILD_PATH)/darwin/arm64/%: $(SOURCES)
	$(call fn_build,darwin,arm64,$*)
## END of building binaries

## BEGIN of tools installation
# usually unnecessary to clean, and may require downloads to restore, so this folder is not automatically cleaned
BIN   := $(shell pwd)/.bin
TOOLS := $(shell pwd)/tools

# helper for executing bins, just `$(BIN_PATH) the_command ...`
BIN_PATH := PATH="$(abspath $(BIN)):$$PATH"

.PHONY: download
download:
	@echo Download go.mod dependencies
	@go mod download

.PHONY: install
install: download ## Install useful CLI tools
	@echo Installing tools from $(TOOLS)/tools.go
	@mkdir -p $(BIN)
	@cd $(TOOLS) && cat tools.go | grep _ | awk -F'"' '{print $$2}' | GOBIN=$(BIN) xargs -tI % go install %
	@touch $(BIN)/.installed

.PHONY: ensure-tools
ensure-tools: ## Ensure tools are installed (runs install only if needed)
	@if [ ! -f $(BIN)/.installed ]; then \
		$(MAKE) install; \
	fi
## END of tools installation

## BEGIN of quality gates
build-dir:
	@mkdir -p $(BUILD_PATH)

.PHONY: test
test: run-lint run-test ## Run all quality gates (unit tests + linting)

.PHONY: test-all
test-all: run-lint run-test-integration ## Run all quality gates including integration tests

.PHONY: cover
cover: run-test ## Test and code coverage
	go tool cover -html=$(COVER_FILE)

.PHONY: fix-lint
fix-lint: ensure-tools ## Auto-fix linting issues
	$(BIN_PATH) golangci-lint run --fix -v ./...

.PHONY: run-lint
run-lint: ensure-tools
	$(BIN_PATH) golangci-lint --version
	$(BIN_PATH) golangci-lint run $(PACKAGES)

.PHONY: run-test
run-test: build-dir ## Run unit tests (skip integration tests)
	go test $(TEST_FLAGS) $(PACKAGES)

.PHONY: run-test-integration
run-test-integration: build-dir ## Run all tests including integration tests (requires Docker)
	go test $(TEST_FLAGS_INTEG) $(PACKAGES)

.PHONY: generate
generate: ensure-tools ## Run go generators
	$(BIN_PATH) go generate ./...

.PHONY: git-status
git-status: ## Check if working directory is clean
	@status=$$(git status --porcelain); \
	if [ ! -z "$${status}" ]; \
	then \
		echo "Error - working directory is dirty. Commit those changes!"; \
		echo "$${status}";\
		exit 1; \
	fi
## END of quality gates

## BEGIN of cleaning
.PHONY: clean
clean: ## Clean up build artifacts
	rm -rfv $(MOCK_PACKAGES) $(BUILD_PATH)
## END of cleaning

## BEGIN of Docker
.PHONY: docker
docker: build.linux ## Build image
	@docker build -t $(APPLICATION):latest .

.PHONY: docker-up
docker-up: ## Start services
	@docker-compose up -d

.PHONY: docker-down
docker-down: ## Stop services
	@docker-compose down

.PHONY: docker-logs
docker-logs: ## Follow logs
	@docker-compose logs -f analyzer
## END of Docker

## BEGIN of Demo
.PHONY: demo
demo: demo-infra demo-run ## Start infrastructure and run analyzer with OTEL

.PHONY: demo-infra
demo-infra: ## Start infrastructure (OTEL Collector, Jaeger, Prometheus, Grafana, Redis)
	@echo "${GREEN}Starting observability infrastructure...${RESET}"
	@docker-compose up -d
	@echo ""
	@echo "${GREEN}Infrastructure started!${RESET}"
	@echo "${CYAN}Services:${RESET}"
	@echo "  OTEL Collector: ${YELLOW}localhost:4318${RESET} (OTLP HTTP)"
	@echo "  Jaeger UI:      ${YELLOW}http://localhost:16686${RESET} (tracing)"
	@echo "  Grafana:        ${YELLOW}http://localhost:3000${RESET} (unified UI)"
	@echo "  Prometheus:     ${YELLOW}http://localhost:9090${RESET}"
	@echo "  Redis:          ${YELLOW}localhost:6379${RESET}"
	@echo ""
	@echo "${CYAN}Architecture:${RESET}"
	@echo "  App → OTLP → Collector → Jaeger (traces) + Prometheus (metrics)"
	@echo ""
	@echo "Use ${YELLOW}make demo-run${RESET} to start the analyzer"

.PHONY: demo-run
demo-run: build ## Run analyzer locally with OTEL enabled
	@echo "${GREEN}Starting Page Analyzer with OpenTelemetry...${RESET}"
	@echo ""
	@echo "${CYAN}Application:${RESET}"
	@echo "  Analyzer UI:   ${YELLOW}http://localhost:8080${RESET}"
	@echo ""
	@echo "${CYAN}View telemetry:${RESET}"
	@echo "  Jaeger UI:      ${YELLOW}http://localhost:16686${RESET}"
	@echo "  Grafana:        ${YELLOW}http://localhost:3000${RESET}"
	@echo ""
	ANALYZER_TRACING_ENABLED=true \
	ANALYZER_OTEL_ENDPOINT=localhost:4318 \
	ANALYZER_METRICS_ENABLED=true \
	ANALYZER_CACHE_MODE=redis \
	ANALYZER_REDIS_ADDR=redis://localhost:6379/0 \
	ANALYZER_LOG_LEVEL=info \
	./$(BUILD_PATH)/$(UNAME_S)/$(GOARCH)/analyzer serve --addr :8080

.PHONY: demo-down
demo-down: ## Stop infrastructure and remove volumes
	@echo "${YELLOW}Stopping infrastructure...${RESET}"
	@docker-compose down -v
	@echo "${GREEN}Infrastructure stopped${RESET}"

.PHONY: demo-logs
demo-logs: ## Follow logs from infrastructure services
	@docker-compose logs -f

.PHONY: demo-status
demo-status: ## Show status of infrastructure services
	@docker-compose ps
## END of Demo

.SUFFIXES:  # Clear built-in suffix rules (avoid implicit rules for old C/C++ files)
