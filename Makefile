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

GIT_HEAD    := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS     := -X main.Version=$(VERSION) -X main.GitHead=$(GIT_HEAD) -s -w
TEST_FLAGS  := -race -count=1 -mod=readonly -cover -coverprofile $(COVER_FILE)
BUILD_FLAGS := -mod=readonly -v

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
## END of tools installation

## BEGIN of quality gates
build-dir:
	@mkdir -p $(BUILD_PATH)

.PHONY: test
test: run-lint run-test ## Run all quality gates (tests, linting, etc.)

.PHONY: cover
cover: run-test ## Test and code coverage
	go tool cover -html=$(COVER_FILE)

.PHONY: fix-lint
fix-lint: ## Auto-fix linting issues
	$(BIN_PATH) golangci-lint run --fix -v ./...

.PHONY: run-lint
run-lint:
	$(BIN_PATH) golangci-lint --version
	$(BIN_PATH) golangci-lint run $(PACKAGES)

.PHONY: run-test
run-test: build-dir
	go test $(TEST_FLAGS) $(PACKAGES)

.PHONY: generate
generate: ## Run go generators
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

.SUFFIXES:  # Clear built-in suffix rules (avoid implicit rules for old C/C++ files)
