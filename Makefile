# Lumen SDK Makefile

# Variables
BINARY_NAME = lumen-hostd
BUILD_DIR = dist
VERSION ?= $(shell git describe --tags --exact-match 2>/dev/null || cat VERSION 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS = -ldflags="-X 'main.Version=$(VERSION)' -X 'main.Commit=$(COMMIT)' -X 'main.BuildTime=$(BUILD_TIME)'"

# Go flags
GO_FLAGS = -v
CGO_ENABLED ?= 0

# Platform specific variables
PLATFORMS = linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

.PHONY: help build build-all build-release archive install install-local uninstall clean clean-deps dev run-hostd release tag show-version set-version quick-start ci ci-fast test test-coverage lint fmt vet deps

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
build: ## Build lumen-hostd for the current platform
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=$(CGO_ENABLED) go build $(GO_FLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/lumen-hostd

build-all: ## Build lumen-hostd for all platforms
	@mkdir -p $(BUILD_DIR)
	@echo "Building for multiple platforms..."
	@for platform in $(PLATFORMS); do \
		os=$$(echo $$platform | cut -d'/' -f1); \
		arch=$$(echo $$platform | cut -d'/' -f2); \
		output_name=$(BINARY_NAME)-$$os-$$arch; \
		if [ $$os = "windows" ]; then \
			output_name=$$output_name.exe; \
		fi; \
		echo "Building $$os/$$arch..."; \
		CGO_ENABLED=$(CGO_ENABLED) GOOS=$$os GOARCH=$$arch go build $(GO_FLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$$output_name ./cmd/lumen-hostd; \
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
			zip -r $(BINARY_NAME)-$(VERSION)-$$os-$$arch.zip $(BINARY_NAME)-$$os-$$arch.exe; \
		else \
			tar -czf $(BINARY_NAME)-$(VERSION)-$$os-$$arch.tar.gz $(BINARY_NAME)-$$os-$$arch; \
		fi; \
	done

# Installation targets
install: build ## Install lumen-hostd to GOPATH/bin
	go install $(LDFLAGS) ./cmd/lumen-hostd

install-local: build ## Install lumen-hostd to /usr/local/bin
	@echo "Installing to /usr/local/bin (requires sudo)"
	sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/

uninstall: ## Remove lumen-hostd from /usr/local/bin
	@echo "Removing from /usr/local/bin (requires sudo)"
	sudo rm -f /usr/local/bin/$(BINARY_NAME)

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

run-hostd: ## Run lumen-hostd in the foreground
	go run $(LDFLAGS) ./cmd/lumen-hostd serve

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

# Version management
show-version: ## Show version information
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
	@echo "Note: Consider creating a git tag: git tag $(VERSION) && git push origin $(VERSION)"

# Quick start
quick-start: build ## Quick build and start
	@echo "Starting lumen-hostd..."
	./$(BUILD_DIR)/$(BINARY_NAME) serve &
	@sleep 2
	@echo "Checking status..."
	./$(BUILD_DIR)/$(BINARY_NAME) version
	@echo "Quick start complete! lumen-hostd running in background."

# CI helpers
ci: deps fmt vet lint test ## Run full CI pipeline

ci-fast: fmt vet test ## Run fast CI pipeline (no linting)
