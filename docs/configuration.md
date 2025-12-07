# Configuration Guide

## Overview

Lumen Hub supports multiple configuration methods:
- **Presets**: Built-in configurations for different use cases
- **Configuration Files**: YAML files for custom settings
- **Environment Variables**: Runtime configuration overrides

### Using Presets (Recommended)

See Presets at [examples](../examples/configs)

```bash
# Start daemon with different presets
lumenhubd --daemon --preset minimal
lumenhubd --daemon --preset lightweight
lumenhubd --daemon --preset basic
lumenhubd --daemon --preset brave

# Or run in foreground
lumenhubd --config --preset basic
```


### Using Configuration Files

```bash
# Start daemon with custom config
lumenhubd --daemon --config /path/to/custom-config.yaml

# Or run in foreground
lumenhubd --config /path/to/custom-config.yaml
```

## Environment Variables

### Core Settings
```bash
LUMEN_REST_HOST=localhost        # REST API host
LUMEN_REST_PORT=5866           # REST API port
LUMEN_LOG_LEVEL=info            # Log level (debug/info/warn/error/fatal)
LUMEN_LOG_FORMAT=json           # Log format (text/json)
LUMEN_LOG_OUTPUT=stdout         # Log output (stdout/stderr/file)
```

### Discovery Settings
```bash
LUMEN_DISCOVERY_ENABLED=true     # Enable/disable discovery
LUMEN_DISCOVERY_SERVICE_TYPE=_lumen._tcp
LUMEN_DISCOVERY_DOMAIN=local
LUMEN_DISCOVERY_SCAN_INTERVAL=30s
LUMEN_DISCOVERY_NODE_TIMEOUT=5m
LUMEN_DISCOVERY_MAX_NODES=20
```

### Connection Settings
```bash
LUMEN_CONNECTION_INSECURE=false
LUMEN_CONNECTION_KEEP_ALIVE=30s
```

### Load Balancer Settings
```bash
LUMEN_LOAD_BALANCER_STRATEGY=round_robin
LUMEN_LOAD_BALANCER_CACHE_ENABLED=true
LUMEN_LOAD_BALANCER_CACHE_TTL=5m
LUMEN_LOAD_BALANCER_DEFAULT_TIMEOUT=30s
LUMEN_LOAD_BALANCER_HEALTH_CHECK=true
LUMEN_LOAD_BALANCER_CHECK_INTERVAL=30s
```

## Configuration Priority

Settings are applied in this order (higher priority overrides lower):

1. **Default values** (hardcoded in application)
2. **Preset configuration** (if `--preset` is used)
3. **Configuration file** (if `--config` is used)
4. **Environment variables** (always override)

## Validation

Configuration is automatically validated on startup. Common validation errors:

### Invalid Log Level
```bash
Error: invalid log level: trace
Valid levels: debug, info, warn, error, fatal
```

### Invalid Port Range
```bash
Error: server rest port must be in range 1-65535
```

### Invalid Time Duration
```bash
Error: discovery scan_interval must be positive
Use duration formats: 30s, 5m, 1h
```

## Troubleshooting

### Configuration Not Loading
```bash
# Check file permissions
ls -la /path/to/config.yaml

# Check YAML syntax
lumenhubd --config /path/to/config.yaml --dry-run
```

### Environment Variables Not Working
```bash
# Check if variables are set
env | grep LUMEN_

# Export variables explicitly
export LUMEN_LOG_LEVEL=debug
lumenhubd --preset basic
```

### Default Settings Applied
```bash
# Show effective configuration
lumenhubd --preset basic --dry-run

# Check which config is being used
lumenhubd --preset basic --verbose
```
