# Configuration Guide

## Overview

`lumen-hostd` (and any application using `pkg/config` directly) is configured via, in increasing priority:

1. **Defaults** (`config.DefaultConfig()`)
2. **A YAML config file** (`lumen-hostd serve --config /path/to/config.yaml`)
3. **Environment variables** (always override both of the above)

```bash
lumen-hostd serve --config /path/to/config.yaml
```

## Configuration file reference

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
  mdns_enabled: true
  broker_url: ""          # set to consume another Broker instead of/alongside mDNS
  hub_url: ""              # deprecated alias for broker_url
  static_nodes: []          # e.g. ["10.0.0.5:50051"]

server:
  rest:
    enabled: true
    host: "0.0.0.0"
    port: 5866
    cors: true

logging:
  level: "info"    # debug | info | warn | error | fatal
  format: "json"   # json | text
  output: "stdout" # stdout | stderr | a file path

chunk:
  enable_auto: true
  threshold: 1048576      # 1 MiB — payloads larger than this get chunked
  max_chunk_bytes: 262144  # 256 KiB per chunk
```

`discovery.broker_url`, `discovery.mdns_enabled`, and `discovery.static_nodes` are additive, not exclusive: every one that's configured runs, and their discovered nodes are merged. At least one must be enabled/set.

`lumen-hostd` itself always runs with an empty internal `broker_url`/`hub_url` regardless of what you set here, to avoid subscribing to itself — see [`lumen-host-implementation-plan.md`](lumen-host-implementation-plan.md) §10. The field still matters for other applications using the SDK directly (e.g. a Lumilio-style consumer connecting *to* a running `lumen-hostd`).

## Environment variables

```bash
LUMEN_DISCOVERY_ENABLED=true
LUMEN_DISCOVERY_SERVICE_TYPE=_lumen._tcp
LUMEN_DISCOVERY_DOMAIN=local
LUMEN_DISCOVERY_DEPLOYMENT_ID=local
LUMEN_DISCOVERY_RESOLVE_TIMEOUT=10s
LUMEN_DISCOVERY_CONNECT_TIMEOUT=10s
LUMEN_DISCOVERY_REDISCOVERY_BACKOFF_MIN=10s
LUMEN_DISCOVERY_REDISCOVERY_BACKOFF_MAX=2m
LUMEN_DISCOVERY_MDNS_ENABLED=true
LUMEN_DISCOVERY_BROKER_URL=http://host.docker.internal:5866
LUMEN_DISCOVERY_HUB_URL=   # deprecated alias for LUMEN_DISCOVERY_BROKER_URL
LUMEN_DISCOVERY_STATIC_NODES=10.0.0.5:50051,10.0.0.6:50051   # comma-separated

LUMEN_REST_HOST=0.0.0.0
LUMEN_REST_PORT=5866

LUMEN_LOG_LEVEL=info
LUMEN_LOG_FORMAT=json
LUMEN_LOG_OUTPUT=stdout
```

There is no environment variable override for the `chunk` section; set it via a config file if you need non-default chunking behavior.

Setting both `LUMEN_DISCOVERY_BROKER_URL`/`broker_url` and the deprecated `LUMEN_DISCOVERY_HUB_URL`/`hub_url` to two *different* non-empty values fails validation at startup — set only `broker_url`.

## Validation

Configuration is validated on startup (`Config.Validate()`). Common errors:

```text
discovery.service_type is required when enabled
discovery.deployment_id is required when enabled
discovery.resolve_timeout must be positive
discovery.rediscovery_backoff_max must be >= rediscovery_backoff_min
rest.port must be in 1-65535
invalid log level: trace
invalid log format: yaml
discovery.broker_url ("...") and deprecated discovery.hub_url ("...") are both set and differ; set only broker_url
```

## Troubleshooting

### Environment variables not taking effect
```bash
env | grep LUMEN_
```
Environment variables always win over the config file — if a setting isn't changing, check whether an env var is overriding it.

### Config file not loading
```bash
ls -la /path/to/config.yaml   # check it exists and is readable
```
`lumen-hostd serve --config` fails fast with a parse or validation error on startup; check the process's stderr/log output (per `logging.output`).

## Known gap

`examples/configs/*.yaml` predates the current schema (it references a `connection:` block and a `max_nodes` field that no longer exist) and needs its own refresh — don't use it as a reference until that happens.
