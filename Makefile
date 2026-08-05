# Lumen SDK development commands

PROTO_BASELINE ?= v1.3.2
PROTO_DIR := proto
HOSTD_BINARY := dist/lumen-hostd
VERSION ?= $(shell git describe --tags --exact-match 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
HOSTD_LDFLAGS := -ldflags="-X 'main.Version=$(VERSION)' -X 'main.Commit=$(COMMIT)' -X 'main.BuildTime=$(BUILD_TIME)'"

.PHONY: help deps fmt-check fmt vet lint test test-coverage build clean ci ci-fast proto-check proto-verify hostd-build hostd-run hostd-install
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

# Host Broker is an experimental discovery bridge. It is source-built only and
# intentionally excluded from the default release/support surface.
hostd-build: ## Build experimental lumen-hostd for the current platform
	@mkdir -p dist
	CGO_ENABLED=0 go build $(HOSTD_LDFLAGS) -o $(HOSTD_BINARY) ./cmd/lumen-hostd

hostd-run: ## Run experimental lumen-hostd in the foreground
	go run $(HOSTD_LDFLAGS) ./cmd/lumen-hostd serve

hostd-install: ## Install experimental lumen-hostd with go install
	go install $(HOSTD_LDFLAGS) ./cmd/lumen-hostd

clean: ## Remove generated local artifacts
	rm -rf dist coverage.out coverage.html

ci: fmt-check vet test ## Run the repository's deterministic default checks

ci-fast: fmt-check test ## Run formatting and tests without vet
