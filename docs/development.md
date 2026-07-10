# Development Guide

This guide covers developing the Lumen SDK project: environment setup, the build system, and contributing guidelines.

## 📋 Table of Contents

- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Project Structure](#project-structure)
- [Makefile Usage](#makefile-usage)
- [Git Workflow](#git-workflow)
- [Development Environment](#development-environment)
- [Testing](#testing)
- [Performance Benchmark Datasets](#performance-benchmark-datasets)
- [Build & Release Process](#build--release-process)
- [Debugging](#debugging)
- [Contributing Guidelines](#contributing-guidelines)

## 🎯 Prerequisites

### Required Tools

- **Go 1.25+** - Install from [golang.org](https://golang.org/dl/)
- **Git** - For version control
- **Make** - Build system (included with most dev environments)

### Optional Development Tools

```bash
# Development hot-reload
go install github.com/cosmtrek/air@latest

# Linting
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

## 🚀 Quick Start

```bash
# Clone the repository
git clone https://github.com/edwinzhancn/lumen-sdk.git
cd lumen-sdk

# Install dependencies
make deps

# Build and run lumen-hostd in the foreground
make build
./dist/lumen-hostd serve
```

In another terminal:
```bash
./dist/lumen-hostd status
curl http://localhost:5866/v1/nodes
```

## 📁 Project Structure

```
lumen-sdk/
├── cmd/
│   ├── lumen-hostd/              # Host Broker daemon and CLI
│   │   ├── cmd/                  # cobra subcommands (serve/install/.../doctor)
│   │   ├── internal/             # internal client/config wiring
│   │   │   └── native/           # per-OS background service management
│   │   └── service/              # daemon lifecycle
│   └── lumen-bench/               # benchmarking harness
├── pkg/
│   ├── client/                   # LumenClient SDK
│   ├── config/                   # Configuration management
│   ├── discovery/                # NodeResolver implementations (mDNS, Broker, static)
│   ├── hostbroker/                # Discovery-only server used by lumen-hostd
│   ├── server/rest/               # General-purpose REST façade (inference + discovery)
│   └── types/                    # Shared task/request/response types
├── docs/                         # Documentation
├── test/                         # Cross-package integration tests
├── examples/client/               # Runnable usage examples per task
├── .github/workflows/            # GitHub Actions
├── Makefile                      # Build system
└── README.md                     # Project overview
```

## Performance Benchmark Datasets

Performance benchmark image datasets are prepared outside the repository. See [Performance Benchmark Dataset Setup](./performance-benchmark-data.md) for the reproducible COCO/CUB download, sampling, and mixed-workload dataset layout.

## 🔧 Makefile Usage

```bash
make help            # show all available targets
make deps             # install dependencies
make dev              # hot-reload development (requires air)
make build            # build lumen-hostd for the current platform
make build-all        # build for linux/darwin/windows, amd64/arm64
make build-release    # build-all with version info from the current git tag
make archive          # build-all + create per-platform .tar.gz/.zip archives
make install          # install to GOPATH/bin
make install-local    # install to /usr/local/bin (sudo)
make uninstall        # remove from /usr/local/bin (sudo)
make clean            # remove dist/ and coverage files
make test             # go test -race with coverage
make test-coverage    # test + open an HTML coverage report
make fmt / vet / lint  # formatting, static analysis, linting
make ci               # fmt + vet + lint + test
make ci-fast          # fmt + vet + test (no linting)
make run-hostd        # go run ./cmd/lumen-hostd serve
make show-version     # print VERSION/COMMIT/BUILD_TIME that a build would embed
make set-version VERSION=v1.2.3   # write the VERSION file
make tag VERSION=v1.2.3           # git tag + push, triggers the release workflow
make release          # clean + test + lint + build-release + archive
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
git checkout main
git pull origin main
git checkout -b feature/your-feature-name
```

#### 2. Make Changes

Use [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add new inference service type
fix: resolve node connection timeout issue
docs: update API documentation
refactor: simplify client configuration
test: add integration tests for streaming
chore: update dependencies
```

#### 3. Keep Branch Updated

```bash
git checkout main
git pull origin main
git checkout feature/your-feature-name
git rebase main
```

#### 4. Create Pull Request

Push your branch and open a PR with a clear description, testing instructions, and any related issue links.

### Release Process

```bash
make set-version VERSION=v1.2.3
git add VERSION
git commit -m "chore: bump version to v1.2.3"
make tag VERSION=v1.2.3
```

Pushing the tag triggers `.github/workflows/release.yml`, which runs tests, cross-compiles `lumen-hostd` for linux/darwin/windows (amd64/arm64, excluding windows/arm64), and publishes a GitHub release with archives attached.

## 🛠 Development Environment

### Local Development Setup

```bash
git clone https://github.com/edwinzhancn/lumen-sdk.git
cd lumen-sdk
make deps
go install github.com/cosmtrek/air@latest
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

### Development Mode

```bash
# Hot reload (requires air)
make dev

# Or run and iterate manually
make run-hostd
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
# Go development
export GO111MODULE=on
export GOPROXY=direct
export GOSUMDB=sum.golang.org
export GOCACHE=$HOME/.cache/go-build
export GOMODCACHE=$HOME/go/pkg/mod
```

`lumen-hostd`'s own runtime configuration is via `LUMEN_*` env vars — see [`configuration.md`](configuration.md), not development-environment variables.

## 🧪 Testing

### Test Types

```bash
# All tests
make test

# A specific package
go test ./pkg/client/...

# Verbose, with race detection
go test -v -race ./...
```

### Test Organization

```
test/
├── config/        # pkg/config black-box tests
└── pkg/
    ├── client/     # pkg/client integration-style tests
    └── types/      # pkg/types tests
```

Package-local `*_test.go` files (white-box tests needing access to unexported state) live alongside their source in `pkg/*` and `cmd/*`, not under `test/`.

### Coverage

```bash
make test-coverage
open coverage.html
```

## 🏗 Build & Release Process

### Local Build Process

```bash
make build                    # dev build, VERSION defaults to git describe or "dev"
VERSION=v1.2.3 make build      # override VERSION
make build-all                 # cross-platform
make build-release archive     # release build + archives
```

### Build Configuration

- `VERSION`: version string (from git tag, `VERSION` file, or `"dev"`)
- `COMMIT`: short git commit hash
- `BUILD_TIME`: build timestamp (UTC)
- `CGO_ENABLED`: default `0`
- `GOOS`/`GOARCH`: target platform

### Version Information

```bash
$ ./dist/lumen-hostd version
Lumen Host Broker v1.2.3
Commit: d612a83
Built: 2026-07-10T08:00:00Z
Go: go1.25.10
OS/Arch: darwin/arm64
```

### Release Automation

1. **Trigger**: push a `v*` git tag
2. **CI**: run tests
3. **Build**: cross-platform `lumen-hostd` binaries with version info injected via `-ldflags`
4. **Release**: create a GitHub release with archives attached
5. **Update**: move the `latest` tag

## 🐛 Debugging

### Debugging the daemon

```bash
# Foreground, with a specific config (set logging.level: debug in the file, or LUMEN_LOG_LEVEL=debug)
LUMEN_LOG_LEVEL=debug ./dist/lumen-hostd serve

# Under the Go debugger
go run ./cmd/lumen-hostd serve &
dlv connect localhost:5866
```

### Debugging the CLI

```bash
# Check what lumen-hostd's own service thinks its state is
./dist/lumen-hostd status

# Diagnose discovery/reachability against a specific config
./dist/lumen-hostd doctor --config /path/to/config.yaml
```

### Common Issues

#### Port already in use
```bash
lsof -i :5866
kill -9 <PID>
# or point at a different port: LUMEN_REST_PORT=5867 ./dist/lumen-hostd serve
```

#### Module issues
```bash
make clean-deps
make deps
go mod tidy
```

#### Build issues
```bash
make clean
make build
go build -v ./cmd/lumen-hostd
```

## 📝 Contributing Guidelines

### Code Style

```bash
make fmt
make lint
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
2. **Create** a feature branch from `main`
3. **Make** changes with clear commits
4. **Test** thoroughly (`make ci`)
5. **Submit** a pull request with a clear title/description, testing instructions, and related issue numbers

### Code Review Checklist

- [ ] Code follows Go conventions
- [ ] Tests are included and passing
- [ ] Documentation is updated
- [ ] No linting errors
- [ ] Version impact considered
- [ ] Breaking changes documented
- [ ] Security implications considered

### Performance Considerations

- **Memory**: monitor memory usage in long-running daemon
- **Concurrency**: use goroutines safely with proper synchronization
- **Network**: handle timeouts and connection pool exhaustion
- **Streaming**: implement proper streaming for large payloads

## 🔗 Additional Resources

### Documentation

- [Main README](../README.md) — project overview
- [Installation Guide](./installation.md) — installing `lumen-hostd`
- [Configuration Guide](./configuration.md) — configuration options
- [Host Broker Implementation Plan](./lumen-host-implementation-plan.md) — architecture and rollout plan

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
