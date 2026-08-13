# Lumen SDK development commands

PROTO_BASELINE ?= v1.3.2
PROTO_DIR := proto
HOSTD_BINARY := dist/lumen-hostd
HOSTD_PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64
VERSION ?= $(shell git describe --tags --exact-match 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
HOSTD_LDFLAGS := -ldflags="-X 'main.Version=$(VERSION)' -X 'main.Commit=$(COMMIT)' -X 'main.BuildTime=$(BUILD_TIME)'"

.PHONY: help deps fmt-check fmt vet lint test test-coverage build clean ci ci-fast proto-check proto-verify hostd-build hostd-build-all hostd-archive hostd-run hostd-install release tag
.DEFAULT_GOAL := help

help: ## Show available targets
	@echo 'Usage: make [target]'
	@echo ''
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z0-9_.-]+:.*?## / {printf "  %-16s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

deps: ## Download and verify Go modules
	go mod download
	go mod verify

fmt-check: ## Fail when committed Go files are not gofmt-formatted
	@test -z "$$(gofmt -l $$(git ls-files '*.go'))" || { \
		echo "unformatted Go files:"; \
		gofmt -l $$(git ls-files '*.go'); \
		exit 1; \
	}

fmt: ## Format committed Go files
	gofmt -w $$(git ls-files '*.go')

vet: ## Run go vet
	go vet ./...

lint: ## Run golangci-lint
	golangci-lint run

test: ## Run the complete test suite with the race detector
	go test -race -coverprofile=coverage.out ./...

test-coverage: test ## Render HTML coverage output
	go tool cover -html=coverage.out -o coverage.html

build: ## Compile the SDK packages (Host Broker is an optional command)
	go build ./pkg/... ./proto/...

# Local proto verification is deterministic once buf/protoc/plugins are installed.
proto-check: ## Lint protos and verify committed generated Go code
	cd $(PROTO_DIR) && buf lint
	@tmp=$$(mktemp -d); trap 'rm -rf $$tmp' EXIT; \
	protoc --proto_path=. --go_out=paths=source_relative:$$tmp --go-grpc_out=paths=source_relative:$$tmp proto/ml_service.proto; \
	for f in proto/ml_service.pb.go proto/ml_service_grpc.pb.go; do \
		diff -u $$f $$tmp/$$f || { \
			echo "generated code out of date: $$f"; \
			echo "regenerate with: protoc --proto_path=. --go_out=paths=source_relative:. --go-grpc_out=paths=source_relative:. proto/ml_service.proto"; \
			exit 1; \
		}; \
	done

proto-verify: proto-check ## Also verify wire compatibility against the pinned remote baseline
	cd $(PROTO_DIR) && buf breaking --against "https://github.com/EdwinZhanCN/Lumen-SDK.git#ref=$(PROTO_BASELINE),subdir=proto"

# Host Broker is an optional discovery bridge. Direct mDNS or static discovery
# remains the default integration path, while release tags still ship hostd.
hostd-build: ## Build optional lumen-hostd for the current platform
	@mkdir -p dist
	CGO_ENABLED=0 go build $(HOSTD_LDFLAGS) -o $(HOSTD_BINARY) ./cmd/lumen-hostd

hostd-build-all: ## Cross-build lumen-hostd for all release platforms
	@set -eu; \
	for platform in $(HOSTD_PLATFORMS); do \
		os=$${platform%/*}; \
		arch=$${platform#*/}; \
		output=dist/$$os-$$arch/lumen-hostd; \
		if [ "$$os" = windows ]; then output=$$output.exe; fi; \
		echo "Building lumen-hostd for $$os/$$arch"; \
		mkdir -p dist/$$os-$$arch; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build $(HOSTD_LDFLAGS) -o $$output ./cmd/lumen-hostd; \
	done

hostd-archive: ## Build and archive versioned lumen-hostd release artifacts
	@if ! printf '%s' "$(VERSION)" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$$'; then \
		echo "Usage: make hostd-archive VERSION=vX.Y.Z"; \
		exit 1; \
	fi
	@$(MAKE) hostd-build-all VERSION=$(VERSION)
	@set -eu; \
	for platform in $(HOSTD_PLATFORMS); do \
		os=$${platform%/*}; \
		arch=$${platform#*/}; \
		if [ "$$os" = windows ]; then \
			(cd dist/$$os-$$arch && zip -q ../lumen-hostd-$(VERSION)-$$os-$$arch.zip lumen-hostd.exe); \
		else \
			tar -czf dist/lumen-hostd-$(VERSION)-$$os-$$arch.tar.gz -C dist/$$os-$$arch lumen-hostd; \
		fi; \
	done

hostd-run: ## Run optional lumen-hostd in the foreground
	go run $(HOSTD_LDFLAGS) ./cmd/lumen-hostd serve

hostd-install: ## Install optional lumen-hostd with go install
	go install $(HOSTD_LDFLAGS) ./cmd/lumen-hostd

release: clean ci hostd-archive ## Validate and create local release artifacts

tag: release ## Create and push a release tag (triggers GitHub Release)
	@if [ -n "$$(git status --porcelain)" ]; then \
		echo "Refusing to tag a dirty worktree"; \
		exit 1; \
	fi
	git tag -a $(VERSION) -m "Release $(VERSION)"
	git push origin refs/tags/$(VERSION)

clean: ## Remove generated local artifacts
	rm -rf dist coverage.out coverage.html

ci: fmt-check vet test ## Run the repository's deterministic default checks

ci-fast: fmt-check test ## Run formatting and tests without vet
