# Development Guide

This guide covers everything you need to know about developing the Lumen SDK project, from setting up your development environment to understanding the build system and contributing guidelines.

## 📋 Table of Contents

- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Project Structure](#project-structure)
- [Makefile Usage](#makefile-usage)
- [Git Workflow](#git-workflow)
- [Development Environment](#development-environment)
- [Testing](#testing)
- [Build & Release Process](#build--release-process)
- [Debugging](#debugging)
- [Contributing Guidelines](#contributing-guidelines)

## 🎯 Prerequisites

### Required Tools

- **Go 1.24+** - Install from [golang.org](https://golang.org/dl/)
- **Git** - For version control
- **Make** - Build system (included with most dev environments)
- **Node.js 16+** - (Optional, for documentation tools)

### Optional Development Tools

```bash
# Development hot-reload
go install github.com/cosmtrek/air@latest

# Linting
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Dependency management
go install github.com/daixiang0/gci@latest
```

## 🚀 Quick Start

```bash
# Clone the repository
git clone https://github.com/edwinzhancn/lumen-sdk.git
cd lumen-sdk

# Install dependencies
make deps

# Build and run a quick test
make quick-start

# Check status
./dist/lumenhub status
./dist/lumenhub node list
```

## 📁 Project Structure

```
lumen-sdk/
├── cmd/                          # Main applications
│   ├── lumenhub/                # CLI client
│   │   ├── cmd/                 # CLI commands
│   │   └── internal/            # CLI internal packages
│   └── lumenhubd/               # Daemon server
│       └── internal/            # Daemon internal packages
├── pkg/                         # Shared libraries
│   ├── client/                  # Client SDK
│   ├── config/                  # Configuration management
│   ├── server/                  # Server components
│   │   └── rest/                # REST API implementation
│   └── ...                      # Other shared packages
├── docs/                        # Documentation
├── test/                        # Integration tests
├── .github/workflows/           # GitHub Actions
├── Makefile                     # Build system
└── README.md                    # Project overview
```

## 🔧 Makefile Usage

The Makefile provides a comprehensive set of targets for development, building, testing, and releasing.

### Core Development Commands

```bash
# Show all available targets
make help

# Install dependencies
make deps

# Run development mode with hot reload
make dev

# Build for current platform
make build

# Run complete CI pipeline locally
make ci
```

### Build Targets

```bash
# Build for current platform
make build

# Build for all platforms (Linux, macOS, Windows)
make build-all

# Build release binaries with version info
make build-release

# Create distribution archives
make archive

# Clean build artifacts
make clean
```

### Installation

```bash
# Install to GOPATH/bin
make install

# Install to /usr/local/bin (requires sudo)
make install-local

# Remove from /usr/local/bin
make uninstall
```

### Testing & Quality

```bash
# Run tests
make test

# Run tests with coverage report
make test-coverage

# Format code
make fmt

# Run static analysis
make vet

# Run linter
make lint

# Run complete CI pipeline
make ci

# Run fast CI pipeline (no linting)
make ci-fast
```

### Version Management

```bash
# Show current version info
make show-version

# Set new version (creates VERSION file)
make set-version VERSION=v1.2.3

# Create and push git tag (triggers release)
make tag VERSION=v1.2.3
```

### Development Helpers

```bash
# Run daemon in foreground with minimal preset
make run-daemon

# Run CLI test (assumes daemon is running)
make run-cli

# Build and start both components
make quick-start
```

### Complete Release

```bash
# Create complete release (clean, test, lint, build, archive)
make release
```

## 🌿 Git Workflow

### Branching Strategy

This project uses a **simple Git flow**:

- **`main`** - Main development branch (always stable)
- **Feature branches** - Created from `main` for new features
- **Tags** - Created from `main` for releases (v1.0.0, v1.0.1, etc.)

### Development Workflow

#### 1. Start New Work

```bash
# Always start from latest main
git checkout main
git pull origin main

# Create feature branch
git checkout -b feature/your-feature-name
# or
git checkout -b fix/issue-description
```

#### 2. Make Changes

```bash
# Make your changes...
git add .
git commit -m "feat: add new inference service support"

# Use conventional commit messages:
# feat: new feature
# fix: bug fix
# docs: documentation changes
# style: formatting changes
# refactor: code refactoring
# test: adding tests
# chore: maintenance tasks
```

#### 3. Keep Branch Updated

```bash
# Keep your branch in sync with main
git checkout main
git pull origin main
git checkout feature/your-feature-name
git rebase main
```

#### 4. Create Pull Request

```bash
# Push your branch
git push origin feature/your-feature-name

# Create pull request on GitHub with:
# - Clear description
# - Link to any issues
# - Testing instructions
```

### Release Process

#### Automated Releases

```bash
# 1. Update version
make set-version VERSION=v1.2.3

# 2. Commit version change
git add VERSION
git commit -m "chore: bump version to v1.2.3"

# 3. Create and push tag (triggers automated release)
make tag VERSION=v1.2.3

# 4. GitHub Actions will:
#    - Run tests
#    - Build binaries for all platforms
#    - Create GitHub release
#    - Upload artifacts
```

#### Manual Development Builds

```bash
# Build without version tag (shows "dev")
make build

# Build with custom version
make build VERSION=dev-feature-branch
```

## 🛠 Development Environment

### Local Development Setup

#### 1. Environment Setup

```bash
# Clone and setup
git clone https://github.com/edwinzhancn/lumen-sdk.git
cd lumen-sdk
make deps

# Set up development tools
go install github.com/cosmtrek/air@latest
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

#### 2. Development Mode

```bash
# Method 1: Hot reload development (recommended)
make dev

# Method 2: Manual daemon + CLI
# Terminal 1: Start daemon
make run-daemon

# Terminal 2: Use CLI
./dist/lumenhub status
./dist/lumenhub node list
```

#### 3. Testing Development Changes

```bash
# Build and test
make build
./dist/lumenhub --version
./dist/lumenhubd --version

# Run integration test
make quick-start
```

### IDE Configuration

#### VS Code

Install these extensions:
- Go (golang.go)
- GitLens (eamodio.gitlens)
- Makefile Tools (ms-vscode.makefile-tools)

**Recommended VS Code settings** (`.vscode/settings.json`):
```json
{
  "go.useLanguageServer": true,
  "go.formatTool": "goimports",
  "go.lintTool": "golangci-lint",
  "go.lintFlags": ["--fast"],
  "go.testOnSave": true,
  "go.coverOnSave": true,
  "go.coverageDecorator": {
    "type": "gutter",
    "coveredHighlightColor": "rgba(64,128,64,0.5)",
    "uncoveredHighlightColor": "rgba(128,64,64,0.25)"
  }
}
```

#### Vim/Neovim

```vim
" .vimrc or init.vim
Plug 'fatih/vim-go'

let g:go_def_mode='gopls'
let g:go_info_mode='gopls'
let g:go_fmt_command = 'goimports'
let g:go_lint_command = 'golangci-lint'
let g:go_test_show_name = 1
```

### Environment Variables

```bash
# Development environment
export LUMENHUB_HOST=localhost
export LUMENHUB_PORT=5866

# Go development
export GO111MODULE=on
export GOPROXY=direct
export GOSUMDB=sum.golang.org

# Optional: Enable Go modules cache
export GOCACHE=$HOME/.cache/go-build
export GOMODCACHE=$HOME/go/pkg/mod
```

## 🧪 Testing

### Test Types

#### 1. Unit Tests

```bash
# Run all tests
make test

# Run tests for specific package
go test ./pkg/client/...

# Run tests with verbose output
go test -v ./...

# Run tests with race detection
go test -race ./...
```

#### 2. Integration Tests

```bash
# Run integration tests
go test -tags=integration ./test/...

# Test CLI with running daemon
./test/cli/run_integration_tests.sh
```

#### 3. Coverage

```bash
# Generate coverage report
make test-coverage

# View coverage in browser
open coverage.html

# Coverage thresholds
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | grep "total:"
```

### Test Organization

```
test/
├── cli/                    # CLI integration tests
│   └── infer_cli_test.go   # CLI inference tests
├── integration/            # Service integration tests
└── e2e/                   # End-to-end tests
```

### Writing Tests

#### Unit Test Example

```go
package client

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
)

func TestNewAPIClient(t *testing.T) {
    client := NewAPIClient("localhost", 5866)

    assert.Equal(t, "http://localhost:5866", client.BaseURL)
    assert.NotNil(t, client.HTTPClient)
    assert.Equal(t, 30*time.Second, client.HTTPClient.Timeout)
}
```

#### Integration Test Example

```go
// +build integration

package test

import (
    "testing"
    "time"
    "github.com/edwinzhancn/lumen-sdk/pkg/client"
)

func TestRealInference(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test in short mode")
    }

    // Test with real daemon
    client := client.NewClient()
    result, err := client.Infer("embedding", "test text")

    assert.NoError(t, err)
    assert.NotNil(t, result)
}
```

## 🏗 Build & Release Process

### Local Build Process

```bash
# Development build (fast, no version info)
make build

# Production build (with version info)
VERSION=v1.2.3 make build

# Cross-platform build
make build-all

# Release build (all platforms + archives)
make build-release archive
```

### Build Configuration

The build system uses these variables:

- `VERSION`: Version string (from git tag, VERSION file, or "dev")
- `COMMIT`: Git commit hash (short form)
- `BUILD_TIME`: Build timestamp (UTC)
- `CGO_ENABLED`: Enable/disable CGO (default: 0)
- `GOOS`: Target operating system
- `GOARCH`: Target architecture

### Version Information

Builds include complete version information:

```bash
$ ./dist/lumenhubd --version
Lumen Hub Daemon v1.0.7
Commit: d612a83
Built: 2025-11-07T08:51:59Z
Go: go1.24.5
OS/Arch: darwin/arm64

$ ./dist/lumenhub --version
lumenhub version v1.0.7 (commit: d612a83, built: 2025-11-07T08:51:59Z)
```

### Release Automation

Releases are fully automated through GitHub Actions:

1. **Trigger**: Push git tag (`v1.2.3`)
2. **CI**: Run tests and build verification
3. **Build**: Cross-platform binaries with version injection
4. **Release**: Create GitHub release with artifacts
5. **Update**: Update `latest` tag

## 🐛 Debugging

### Debugging Daemon

```bash
# Run daemon with debug logging
./dist/lumenhubd --preset minimal --log-level debug

# Run daemon in foreground
./dist/lumenhubd --preset minimal --daemon=false

# Debug with dlv (Go debugger)
go run ./cmd/lumenhubd --preset minimal &
dlv connect localhost:5866
```

### Debugging CLI

```bash
# Verbose CLI output
./dist/lumenhub --verbose status

# Debug API calls
export LUMENHUB_DEBUG=1
./dist/lumenhub status

# Test with specific host/port
./dist/lumenhub --host localhost --port 5866 status
```

### Common Issues

#### Port Already in Use

```bash
# Find process using port 5866
lsof -i :5866

# Kill process
kill -9 <PID>

# Or use different port
LUMENHUB_PORT=5867 ./dist/lumenhubd --preset minimal
```

#### Module Issues

```bash
# Clean module cache
make clean-deps

# Re-download dependencies
make deps

# Update dependencies
go mod tidy
go mod download
```

#### Build Issues

```bash
# Clean build
make clean
make build

# Build with verbose output
go build -v ./cmd/lumenhubd
go build -v ./cmd/lumenhub
```

## 📝 Contributing Guidelines

### Code Style

This project follows Go conventions and uses automated tools:

```bash
# Format code
make fmt

# Run linter
make lint

# Run static analysis
make vet
```

### Commit Messages

Use [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add new inference service type
fix: resolve node connection timeout issue
docs: update API documentation
style: format Go code
refactor: simplify client configuration
test: add integration tests for streaming
chore: update dependencies
```

### Pull Request Process

1. **Fork** the repository
2. **Create** feature branch from main
3. **Make** changes with proper commits
4. **Test** thoroughly (`make ci`)
5. **Submit** pull request with:
   - Clear title and description
   - Testing instructions
   - Related issue numbers
   - Screenshots if applicable

### Code Review Checklist

- [ ] Code follows Go conventions
- [ ] Tests are included and passing
- [ ] Documentation is updated
- [ ] No linting errors
- [ ] Version impact considered
- [ ] Breaking changes documented
- [ ] Security implications considered

### Performance Considerations

- **Memory**: Monitor memory usage in long-running daemon
- **Concurrency**: Use goroutines safely with proper synchronization
- **Network**: Handle network timeouts and connection pools
- **Streaming**: Implement proper streaming for large payloads

### Security Guidelines

- **Input validation**: Validate all user inputs
- **Error handling**: Don't expose sensitive information in errors
- **Authentication**: Use secure authentication methods
- **Dependencies**: Keep dependencies updated for security patches

## 🔗 Additional Resources

### Documentation

- [Main README](../README.md) - Project overview
- [CLI README](../cmd/lumenhub/README.md) - CLI usage guide
- [API Documentation](../docs/api.md) - REST API reference
- [Configuration Guide](../docs/configuration.md) - Configuration options

### Go Resources

- [Effective Go](https://golang.org/doc/effective_go.html)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Go Modules Reference](https://golang.org/cmd/go/#hdr-Modules__module_maintenance_and_more)

### Tools

- [golangci-lint](https://golangci-lint.run/) - Go linter
- [air](https://github.com/cosmtrek/air) - Live reload for Go
- [dlv](https://github.com/go-delve/delve) - Go debugger
- [gofmt](https://golang.org/cmd/gofmt/) - Go formatter

---

For questions or help with development, please:
1. Check existing [Issues](https://github.com/edwinzhancn/lumen-sdk/issues)
2. Create a new issue with detailed information
3. Join discussions in existing issues and pull requests