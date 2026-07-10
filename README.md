# Lumen SDK

Lumen SDK is a Go toolkit for discovering and calling distributed ML inference nodes ("Lumen nodes") over gRPC — mDNS-based discovery, a task-aware connection pool, and a REST façade for non-Go clients.

`lumen-hostd` is the SDK's Host Broker: a small background service that discovers Lumen nodes on the host's LAN via mDNS and republishes them over a WebSocket, for applications that can't do their own local-network discovery — most commonly a containerized app running under Docker Desktop on macOS or Windows, where the container's bridge network can't see LAN multicast traffic.

```text
Containerized app
    │
    │ ordinary TCP/WebSocket (host.docker.internal)
    ▼
lumen-hostd on the host OS
    │
    │ mDNS on physical network interfaces
    ▼
Lumen inference nodes on the LAN
```

`lumen-hostd` is a discovery-only control-plane process. It never proxies inference payloads — once a node is discovered, applications using the Go SDK connect to it directly over gRPC.

---

## Repository layout

```text
pkg/
├── client/       LumenClient: discovery + gRPC connection pool + inference calls
├── config/       Configuration loading, validation, env var overrides
├── discovery/    NodeResolver implementations: mDNS, Broker push, static nodes
├── hostbroker/   Discovery-only HTTP/WebSocket server used by lumen-hostd
├── server/rest/  General-purpose REST façade (inference + discovery), for embedding
└── types/        Shared task/request/response types

cmd/
├── lumen-hostd/  The Host Broker daemon and CLI
└── lumen-bench/  Benchmarking harness

docs/                          Guides referenced below
examples/client/               Minimal usage examples per task type
```

## Using the Go SDK directly

```go
cfg := config.DefaultConfig() // mDNS enabled by default
c, err := client.NewLumenClient(cfg, logger)
if err != nil {
    log.Fatal(err)
}
if err := c.Start(ctx); err != nil {
    log.Fatal(err)
}
defer c.Close()

resp, err := c.Infer(ctx, &pb.InferRequest{
    Task:        "semantic_text_embed",
    Payload:     []byte("hello world"),
    PayloadMime: "text/plain",
})
```

See `pkg/client/README.md` for discovery backend configuration and pool behavior, and `examples/client/` for complete runnable examples per task.

## Running lumen-hostd

Download a release binary from [GitHub Releases](https://github.com/EdwinZhanCN/Lumen-SDK/releases), or build from source:

```bash
make build
./dist/lumen-hostd version
```

Run it as a background service (installs a per-user LaunchAgent on macOS, a systemd user unit on Linux, or a Task Scheduler entry on Windows):

```bash
lumen-hostd install    # registers and starts the service
lumen-hostd status     # check install/running state
lumen-hostd doctor     # diagnose discovery and reachability issues
lumen-hostd uninstall  # remove the service
```

Or run it in the foreground (e.g. in a container, or for local development):

```bash
lumen-hostd serve
```

Point an application at it with `LUMEN_DISCOVERY_BROKER_URL=http://host.docker.internal:5866` (see `docs/configuration.md`).

## Documentation

- [`docs/installation.md`](docs/installation.md) — installing `lumen-hostd`
- [`docs/configuration.md`](docs/configuration.md) — config file and environment variable reference
- [`docs/development.md`](docs/development.md) — building, testing, and contributing
- [`docs/lumen-host-implementation-plan.md`](docs/lumen-host-implementation-plan.md) — the Host Broker architecture and rollout plan
- [`pkg/client/README.md`](pkg/client/README.md), [`pkg/config/README.md`](pkg/config/README.md), [`pkg/server/rest/README.md`](pkg/server/rest/README.md) — per-package reference
