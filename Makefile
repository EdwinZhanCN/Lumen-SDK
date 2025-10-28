# Lumen SDK Makefile

# Variables
BINARY_NAME_LUMENHUBD = lumenhubd
BINARY_NAME_LUMENHUB = lumenhub
BUILD_DIR = dist
VERSION ?= $(shell cat VERSION 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS_LUMENHUBD = -ldflags="-X 'main.Version=$(VERSION)' -X 'main.Commit=$(COMMIT)' -X 'main.BuildTime=$(BUILD_TIME)'"
LDFLAGS_LUMENHUB = -ldflags="-X 'Lumen-SDK/cmd/lumenhub/cmd.Version=$(VERSION)' -X 'Lumen-SDK/cmd/lumenhub/cmd.Commit=$(COMMIT)' -X 'Lumen-SDK/cmd/lumenhub/cmd.BuildTime=$(BUILD_TIME)'"

# Go flags
GO_FLAGS = -v
CGO_ENABLED ?= 0

# Platform specific variables
PLATFORMS = linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

.PHONY: help build clean test lint fmt vet deps install release docker-build docker-run

# Default target
.DEFAULT_GOAL := help

# Help target
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Development targets
deps: ## Download dependencies
	go mod download
	go mod verify

fmt: ## Format Go code
	go fmt ./...

vet: ## Vet Go code
	go vet ./...

lint: ## Run linter
	golangci-lint run

test: ## Run tests
	go test -v -race -coverprofile=coverage.out ./...

test-coverage: test ## Show test coverage
	go tool cover -html=coverage.out -o coverage.html

# Build targets
build: ## Build binaries for current platform
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=$(CGO_ENABLED) go build $(GO_FLAGS) $(LDFLAGS_LUMENHUBD) -o $(BUILD_DIR)/$(BINARY_NAME_LUMENHUBD) ./cmd/lumenhubd
	CGO_ENABLED=$(CGO_ENABLED) go build $(GO_FLAGS) $(LDFLAGS_LUMENHUB) -o $(BUILD_DIR)/$(BINARY_NAME_LUMENHUB) ./cmd/lumenhub

build-all: ## Build binaries for all platforms
	@mkdir -p $(BUILD_DIR)
	@echo "Building for multiple platforms..."
	@for platform in $(PLATFORMS); do \
		os=$$(echo $$platform | cut -d'/' -f1); \
		arch=$$(echo $$platform | cut -d'/' -f2); \
		output_name_lumenhubd=$(BINARY_NAME_LUMENHUBD)-$$os-$$arch; \
		output_name_lumenhub=$(BINARY_NAME_LUMENHUB)-$$os-$$arch; \
		if [ $$os = "windows" ]; then \
			output_name_lumenhubd=$$output_name_lumenhubd.exe; \
			output_name_lumenhub=$$output_name_lumenhub.exe; \
		fi; \
		echo "Building $$os/$$arch..."; \
		CGO_ENABLED=$(CGO_ENABLED) GOOS=$$os GOARCH=$$arch go build $(GO_FLAGS) $(LDFLAGS_LUMENHUBD) -o $(BUILD_DIR)/$$output_name_lumenhubd ./cmd/lumenhubd; \
		CGO_ENABLED=$(CGO_ENABLED) GOOS=$$os GOARCH=$$arch go build $(GO_FLAGS) $(LDFLAGS_LUMENHUB) -o $(BUILD_DIR)/$$output_name_lumenhub ./cmd/lumenhub; \
	done

build-release: VERSION ?= $(shell git describe --tags --exact-match 2>/dev/null || echo $(VERSION))
build-release: ## Build release binaries with version info
	@echo "Building release version: $(VERSION)"
	$(MAKE) build-all

# Archive targets
archive: build-all ## Create archives for distribution
	@echo "Creating archives..."
	@cd $(BUILD_DIR); \
	for platform in $(PLATFORMS); do \
		os=$$(echo $$platform | cut -d'/' -f1); \
		arch=$$(echo $$platform | cut -d'/' -f2); \
		if [ $$os = "windows" ]; then \
			zip -r lumenhub-$(VERSION)-$$os-$$arch.zip $(BINARY_NAME_LUMENHUBD)-$$os-$$arch.exe $(BINARY_NAME_LUMENHUB)-$$os-$$arch.exe; \
		else \
			tar -czf lumenhub-$(VERSION)-$$os-$$arch.tar.gz $(BINARY_NAME_LUMENHUBD)-$$os-$$arch $(BINARY_NAME_LUMENHUB)-$$os-$$arch; \
		fi; \
	done

# Installation targets
install: build ## Install binaries to GOPATH/bin
	go install $(LDFLAGS_LUMENHUBD) ./cmd/lumenhubd
	go install $(LDFLAGS_LUMENHUB) ./cmd/lumenhub

install-local: build ## Install binaries to /usr/local/bin
	@echo "Installing to /usr/local/bin (requires sudo)"
	sudo cp $(BUILD_DIR)/$(BINARY_NAME_LUMENHUBD) /usr/local/bin/
	sudo cp $(BUILD_DIR)/$(BINARY_NAME_LUMENHUB) /usr/local/bin/

uninstall: ## Remove binaries from /usr/local/bin
	@echo "Removing from /usr/local/bin (requires sudo)"
	sudo rm -f /usr/local/bin/$(BINARY_NAME_LUMENHUBD)
	sudo rm -f /usr/local/bin/$(BINARY_NAME_LUMENHUB)

# Cleanup targets
clean: ## Clean build artifacts
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

clean-deps: ## Clean module cache
	go clean -modcache

# Development helpers
dev: ## Run in development mode with auto-reload
	@if command -v air >/dev/null 2>&1; then \
		air -c .air.toml; \
	else \
		echo "air not found. Install with: go install github.com/cosmtrek/air@latest"; \
	fi

run-daemon: ## Run daemon in foreground with minimal preset
	go run $(LDFLAGS_LUMENHUBD) ./cmd/lumenhubd --preset minimal

run-cli: ## Run CLI (assumes daemon is running)
	go run $(LDFLAGS_LUMENHUB) ./cmd/lumenhub status

# Release targets
release: clean test lint build-release archive ## Create a complete release

tag: ## Create and push a new git tag
	@if [ -z "$(VERSION)" ]; then \
		echo "Usage: make tag VERSION=v1.2.3"; \
		exit 1; \
	fi
	@if [ -z "$(COMMIT)" ]; then \
		echo "No git repository found"; \
		exit 1; \
	fi
	@echo "Creating tag $(VERSION) for commit $(COMMIT)"
	git tag -a $(VERSION) -m "Release $(VERSION)"
	git push origin $(VERSION)
	@echo "Tag $(VERSION) pushed. GitHub Actions will build and create release."

# Docker targets (if needed in future)
docker-build: ## Build Docker image for testing
	docker build -t lumenhub:test .

docker-run: ## Run Docker container for testing
	docker run -d --name lumenhub-test -p 8080:8080 lumenhub:test

docker-stop: ## Stop and remove test container
	docker stop lumenhub-test || true
	docker rm lumenhub-test || true

# Version management
version: ## Show version information
	@echo "Version: $(VERSION)"
	@echo "Commit: $(COMMIT)"
	@echo "Build Time: $(BUILD_TIME)"

set-version: ## Set new version (usage: make set-version VERSION=v1.2.3)
	@if [ -z "$(VERSION)" ]; then \
		echo "Usage: make set-version VERSION=v1.2.3"; \
		exit 1; \
	fi
	@echo "$(VERSION)" > VERSION
	@echo "Version updated to $(VERSION)"

# Quick start
quick-start: ## Quick build and start
	$(MAKE) build
	@echo "Starting lumenhubd daemon..."
	./$(BUILD_DIR)/$(BINARY_NAME_LUMENHUBD) --preset basic &
	@sleep 2
	@echo "Testing CLI..."
	./$(BUILD_DIR)/$(BINARY_NAME_LUMENHUB) --version
	./$(BUILD_DIR)/$(BINARY_NAME_LUMENHUB) status
	@echo "Quick start complete! Daemon running in background."

# CI helpers
ci: deps fmt vet lint test ## Run full CI pipeline

ci-fast: fmt vet test ## Run fast CI pipeline (no linting)
