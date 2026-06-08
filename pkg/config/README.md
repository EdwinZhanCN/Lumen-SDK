# Config Module

## Overview

Provides unified configuration management for the Lumen SDK. Supports YAML file loading, environment variable overrides, and inline validation.

## Module Structure

```
pkg/config/
├── config.go    # Config types, loading, validation, saving
├── defaults.go  # DefaultConfig() with sensible defaults
└── README.md
```

**Configuration hierarchy**:
```
Config
├── Discovery   (service discovery: mDNS, hub URL)
├── Server
│   └── REST    (REST API server)
├── Logging     (level, format, output)
└── Chunk       (payload chunking)
```

## Core Types

| Type              | Purpose                                       |
|-------------------|-----------------------------------------------|
| `Config`          | Top-level config, contains all sub-configs     |
| `DiscoveryConfig` | mDNS / Hub push discovery settings             |
| `ServerConfig`    | Server settings (REST)                         |
| `RESTConfig`      | REST API host, port, CORS                      |
| `LoggingConfig`   | Log level, format, output                      |
| `ChunkConfig`     | Automatic payload chunking thresholds          |

## Usage

### Load from YAML

```go
cfg, err := config.LoadConfig("config.yaml")
if err != nil {
    log.Fatal(err)
}
```

### Use defaults

```go
cfg := config.DefaultConfig()
```

### Environment variable overrides

```bash
export LUMEN_DISCOVERY_ENABLED=true
export LUMEN_DISCOVERY_MDNS_ENABLED=true
export LUMEN_DISCOVERY_DEPLOYMENT_ID=local
export LUMEN_DISCOVERY_RESOLVE_TIMEOUT=10s
export LUMEN_DISCOVERY_CONNECT_TIMEOUT=10s
export LUMEN_DISCOVERY_REDISCOVERY_BACKOFF_MIN=10s
export LUMEN_DISCOVERY_REDISCOVERY_BACKOFF_MAX=2m
export LUMEN_DISCOVERY_HUB_URL=http://hub:5866
export LUMEN_REST_HOST=0.0.0.0
export LUMEN_REST_PORT=5866
export LUMEN_LOG_LEVEL=debug
export LUMEN_LOG_FORMAT=json
export LUMEN_LOG_OUTPUT=stdout
```

### YAML example

```yaml
discovery:
  enabled: true
  service_type: "_lumen._tcp"
  domain: "local"
  deployment_id: "local"
  resolve_timeout: 10s
  connect_timeout: 10s
  rediscovery_backoff_min: 10s
  rediscovery_backoff_max: 2m
  max_nodes: 20
  mdns_enabled: true
  hub_url: ""

server:
  rest:
    enabled: true
    host: "0.0.0.0"
    port: 5866
    cors: true

logging:
  level: "info"
  format: "json"
  output: "stdout"

chunk:
  enable_auto: true
  threshold: 1048576      # 1 MiB
  max_chunk_bytes: 262144  # 256 KiB
```

### Validation

```go
if err := cfg.Validate(); err != nil {
    log.Fatal(err)
}
```

Validates:
- Discovery fields (`service_type`, `deployment_id`, resolve/connect timeouts, rediscovery backoff) when enabled
- Deprecated discovery fields (`scan_interval`, `node_timeout`) are accepted for compatibility when non-negative
- REST port range (1–65535) when REST is enabled
- Log level (`debug`, `info`, `warn`, `error`, `fatal`)
- Log format (`json`, `text`)

### Save config

```go
err := cfg.SaveConfig("config.yaml")
```
