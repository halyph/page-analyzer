# Refactoring Plan - Polish Before Phase 10

**Goal:** Improve code quality and development experience based on best practices from lounge-campaign-creator project.

---

## 1. Simplify Configuration Management (1 hour)

### Problem
Current `internal/config/env.go` (146 lines) has manual parsing functions that are verbose and error-prone:
- `getEnv()`, `getEnvInt()`, `getEnvInt64()`, `getEnvBool()`, `getEnvDuration()`
- No panic on invalid values (silent failures)
- Inconsistent error handling

### Solution: Adopt envygo Pattern

**Create `internal/envutil/env.go`** (similar to envygo):
```go
package envutil

// EnvString returns env var value or fallback
func EnvString(key, fallback string) string

// EnvInt panics if value invalid
func EnvInt(key string, fallback int) int

// EnvBool panics if value invalid
func EnvBool(key string, fallback bool) bool

// EnvDuration panics if value invalid
func EnvDuration(key string, fallback time.Duration) time.Duration
```

**Simplify `internal/config/env.go`:**
```go
func LoadFromEnv() Config {
    cfg := Defaults()

    // Server
    cfg.Server.Addr = envutil.EnvString("ANALYZER_ADDR", cfg.Server.Addr)
    cfg.Server.ReadTimeout = envutil.EnvDuration("ANALYZER_READ_TIMEOUT", cfg.Server.ReadTimeout)

    // Much cleaner!
    return cfg
}
```

**Benefits:**
- ✅ 50% less code in env.go
- ✅ Panics on invalid values (fail fast)
- ✅ Reusable utility
- ✅ Cleaner, more maintainable

**Files to modify:**
- ❌ Create `internal/envutil/env.go` (~100 lines)
- ❌ Create `internal/envutil/env_test.go`
- ✏️ Simplify `internal/config/env.go` (146 → ~80 lines)
- ✏️ Update `internal/config/env_test.go` (keep existing tests)

---

## 2. Enhance Makefile (1 hour)

### Current Issues
- No colors (hard to scan output)
- No help documentation
- No tools management
- No multi-arch build support
- No linter integration

### Solution: Adopt lounge-campaign-creator Makefile Pattern

**Add color support:**
```makefile
GREEN  := $(shell tput -Txterm setaf 2)
YELLOW := $(shell tput -Txterm setaf 3)
CYAN   := $(shell tput -Txterm setaf 6)
RESET  := $(shell tput -Txterm sgr0)
```

**Add help target:**
```makefile
.PHONY: help
help: ## Show this help
	@echo ''
	@echo 'Usage:'
	@echo '  ${YELLOW}make${RESET} ${GREEN}<target>${RESET}'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} { \
		if (/^[a-zA-Z_-]+:.*?##.*$$/) {printf "    ${YELLOW}%-20s${GREEN}%s${RESET}\\n", $$1, $$2} \
		else if (/^## .*$$/) {printf "  ${CYAN}%s${RESET}\\n", substr($$1,4)} \
		}' $(MAKEFILE_LIST)
```

**Add build functions:**
```makefile
# Detect OS and architecture
UNAME_S := $(shell uname -s | tr A-Z a-z)
UNAME_M := $(shell uname -m)

# Build for current platform
define fn_build
@echo "${GREEN}Building $(1) for ${UNAME_S}/${UNAME_M}${RESET}"
CGO_ENABLED=0 go build $(BUILD_FLAGS) -ldflags="$(LD_FLAGS)" -o $(BUILD_PATH)/$(1) ./cmd
@echo "${GREEN}✓ $(1) build complete${RESET}"
endef

# Build for linux (amd64 + arm64)
define fn_build_linux
@echo "${GREEN}Building $(1) for linux/amd64${RESET}"
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(BUILD_FLAGS) -ldflags="$(LD_FLAGS)" -o $(BUILD_PATH)/linux/amd64/$(1) ./cmd
@echo "${GREEN}✓ $(1) for linux/amd64 complete${RESET}"

@echo "${GREEN}Building $(1) for linux/arm64${RESET}"
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build $(BUILD_FLAGS) -ldflags="$(LD_FLAGS)" -o $(BUILD_PATH)/linux/arm64/$(1) ./cmd
@echo "${GREEN}✓ $(1) for linux/arm64 complete${RESET}"
endef
```

**Add tools management:**
```makefile
BIN   := $(shell pwd)/.bin
TOOLS := $(shell pwd)/tools

BIN_PATH := PATH="$(abspath $(BIN)):$$PATH"

.PHONY: install
install: download ## Install dev tools
	@echo "${GREEN}Installing tools from ${TOOLS}/tools.go${RESET}"
	@cd $(TOOLS) && cat tools.go | grep _ | awk -F'"' '{print $$2}' | GOBIN=$(BIN) xargs -tI % go install %
```

**Add linter:**
```makefile
.PHONY: lint
lint: ## Run golangci-lint
	@echo "${GREEN}Running linter...${RESET}"
	$(BIN_PATH) golangci-lint run ./...
```

**New Makefile structure:**
```makefile
# Variables
VERSION := $(shell git describe --tags --always --dirty)
GIT_HEAD := $(shell git rev-parse --short HEAD)
BUILD_PATH := build
LD_FLAGS := -X main.Version=$(VERSION) -X main.GitHead=$(GIT_HEAD)

# Colors
GREEN  := $(shell tput -Txterm setaf 2)
YELLOW := $(shell tput -Txterm setaf 3)
RESET  := $(shell tput -Txterm sgr0)

## Help:
.PHONY: help
help: ## Show this help
	# implementation

## Development:
.PHONY: install
install: ## Install dev tools

.PHONY: lint
lint: ## Run linter

.PHONY: test
test: ## Run tests

.PHONY: cover
cover: ## Show coverage

## Build:
.PHONY: build
build: ## Build for current platform
	$(call fn_build,analyzer)

.PHONY: build.linux
build.linux: ## Build for linux (amd64 + arm64)
	$(call fn_build_linux,analyzer)

## Docker:
.PHONY: docker
docker: build.linux ## Build Docker image
```

**Files to create:**
- ❌ Create `tools/tools.go` (golangci-lint)
- ❌ Create `tools/go.mod` (separate module)
- ✏️ Enhance `Makefile` (current 28 lines → ~150 lines with colors, help, functions)
- ❌ Create `.golangci.yml` (linter config)

---

## 3. Simplify Docker Build (30 min)

### Problem
Current approach uses multi-stage build:
```dockerfile
FROM golang:1.25-alpine AS builder
# ... build inside Docker
FROM alpine:latest
COPY --from=builder /app/analyzer .
```

**Issues:**
- ❌ Slow (downloads deps every time unless cached)
- ❌ Requires Docker to have network access
- ❌ Can't leverage local Go module cache
- ❌ Hard to debug build issues
- ❌ Multi-arch requires complex buildx setup

### Solution: Pre-build Binary, Copy to Docker

**New approach (like lounge-campaign-creator):**

```dockerfile
# Dockerfile
ARG TARGETARCH=amd64
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata wget

WORKDIR /app

# Copy pre-built binary (built outside Docker)
COPY build/linux/${TARGETARCH}/analyzer .

USER nobody

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/api/health || exit 1

ENTRYPOINT ["./analyzer"]
CMD ["serve", "--addr", ":8080"]
```

**Build workflow:**
```bash
# Local development
make build              # Build for current platform
./bin/analyzer serve

# Docker (single arch)
make build.linux        # Build linux/amd64 and linux/arm64
docker build -t page-analyzer:latest .

# Docker (multi-arch with buildx)
make build.linux
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  --tag page-analyzer:latest \
  --push .
```

**Benefits:**
- ✅ **10x faster builds** (no Go compilation in Docker)
- ✅ Leverages local Go module cache
- ✅ Easy to debug (build locally first)
- ✅ Multi-arch native support via COPY
- ✅ Smaller image (no build tools)
- ✅ Works offline (pre-built binary)

**Files to modify:**
- ✏️ Simplify `Dockerfile` (45 → 20 lines)
- ❌ Create `.dockerignore`
- ✏️ Update `docker-compose.yml` (add build context)

---

## 4. Add Development Tools Setup (30 min)

### Create tools/tools.go Pattern

```go
//go:build tools

package tools

import (
	_ "github.com/golangci/golangci-lint/cmd/golangci-lint"
)
```

**Separate go.mod:**
```go
// tools/go.mod
module github.com/halyph/page-analyzer/tools

go 1.25

require (
	github.com/golangci/golangci-lint v1.61.0
)
```

**Installation:**
```bash
make install   # Installs to .bin/ directory
make lint      # Uses .bin/golangci-lint
```

**Benefits:**
- ✅ Version-controlled tools
- ✅ Consistent across team
- ✅ No global installs
- ✅ Isolated from main project

**Files to create:**
- ❌ Create `tools/tools.go`
- ❌ Create `tools/go.mod`
- ❌ Create `.golangci.yml` config

---

## 5. Add .dockerignore (5 min)

Currently Docker copies entire context (including .git, build artifacts).

**Create `.dockerignore`:**
```
# Git
.git
.github
.gitignore

# Build artifacts
build/
bin/
.bin/
*.log
*.out

# IDE
.vscode/
.idea/
*.swp
*.swo

# Test
coverage.txt
*.test

# Documentation
*.md
!README.md

# Docker
docker-compose*.yml
Dockerfile*

# CI/CD
.claude/
.zappr.yaml
delivery.yaml

# OS
.DS_Store
Thumbs.db
```

**Benefits:**
- ✅ Faster Docker builds (smaller context)
- ✅ Smaller images
- ✅ Security (no .git, credentials)

---

## Implementation Order

### Phase 1: Foundation (1 hour)
1. ✅ Create `internal/envutil/env.go` + tests
2. ✅ Simplify `internal/config/env.go` using envutil
3. ✅ Run tests to verify no regressions
4. ✅ Commit: "refactor: simplify config with envutil package"

### Phase 2: Build System (1 hour)
1. ✅ Create `tools/tools.go` + `tools/go.mod`
2. ✅ Create `.golangci.yml`
3. ✅ Enhance Makefile with colors, help, functions, lint
4. ✅ Test: `make help`, `make install`, `make lint`, `make build.linux`
5. ✅ Commit: "feat: enhance Makefile with colors, help, tools management"

### Phase 3: Docker (30 min)
1. ✅ Simplify `Dockerfile` (single-stage, copy binary)
2. ✅ Create `.dockerignore`
3. ✅ Update docker-compose.yml if needed
4. ✅ Test: `make build.linux && docker build .`
5. ✅ Commit: "refactor: simplify Docker to single-stage build"

### Phase 4: Testing & Documentation (30 min)
1. ✅ Run full test suite
2. ✅ Test Docker build and run
3. ✅ Update TODO.md with completed items
4. ✅ Commit: "docs: update TODO after refactoring"

---

## Expected Results

### Code Metrics
- `internal/config/env.go`: 146 → ~80 lines (45% reduction)
- `Dockerfile`: 45 → ~20 lines (55% reduction)
- New: `internal/envutil/env.go`: ~100 lines (reusable)
- New: `.golangci.yml`: ~50 lines
- Enhanced: `Makefile`: 28 → ~150 lines (with colors, help, functions)

### Developer Experience
- ✅ `make help` - See all available commands
- ✅ `make install` - Install dev tools locally
- ✅ `make lint` - Run linter
- ✅ `make test` - Run tests with coverage
- ✅ `make build` - Build for current platform
- ✅ `make build.linux` - Build for Linux (amd64 + arm64)
- ✅ Colored output for better readability
- ✅ Faster Docker builds (pre-built binaries)

### Build Performance
- Docker build time: **5 minutes → 10 seconds** (30x faster)
- Local builds leverage Go module cache
- Multi-arch builds native support

---

## Total Estimated Time: **3 hours**

1. Phase 1: Configuration refactoring - 1 hour
2. Phase 2: Build system enhancement - 1 hour
3. Phase 3: Docker simplification - 30 min
4. Phase 4: Testing & docs - 30 min

---

## Benefits Summary

✅ **Code Quality**
- Cleaner config management (envutil pattern)
- Fail-fast on invalid config (panic on parse errors)
- Better linting with golangci-lint

✅ **Developer Experience**
- Colored Makefile output
- Self-documenting `make help`
- Version-controlled dev tools
- Faster feedback loop

✅ **Build Performance**
- 30x faster Docker builds
- Native multi-arch support
- Offline-capable builds

✅ **Maintainability**
- Less code to maintain
- Industry-standard patterns
- Better separation of concerns

---

## After This Refactoring

Once complete, we'll have a **production-ready build system** and can proceed with:
- ✅ Phase 6: Redis cache implementation
- ✅ Phase 10: OpenTelemetry observability
- ✅ Phase 11: Full observability stack
- ✅ Phase 12: Documentation
