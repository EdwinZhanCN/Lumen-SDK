# Client Module

## Overview

The client module provides a high-performance inference client for distributed ML nodes. It uses an **event-driven** architecture: discovery events drive connection management, gRPC native connectivity state drives health, and inference failures provide immediate feedback.

## Key Features

- **Event-driven discovery** via `NodeResolver` interface (mDNS or Broker push)
- **gRPC-native health monitoring** — no polling, no timers, no health RPCs
- **Task-aware round-robin** node selection
- **Lock-free metrics** via atomic counters
- **Automatic payload chunking** for large requests

## Architecture

```
NodeResolver (mDNS / Broker)
    │
    │  NodeEvent stream
    ▼
  Pool ──── gRPC connectivity.State ──── healthy/unhealthy subsets
    │
    │  Pick(task) → round-robin
    ▼
LumenClient.Infer() ──► gRPC bidirectional stream ──► ML Node
```

## Module Structure

```
pkg/client/
├── client.go    # LumenClient: composes Pool + NodeResolver
├── pool.go      # Pool: event-driven gRPC connection management
├── chunker.go   # Payload chunking utility
├── logger.go    # ensureLogger helper
└── README.md
```

## Core Types

| Type            | Purpose                                              |
|-----------------|------------------------------------------------------|
| `LumenClient`   | Main client: inference, metrics, node listing         |
| `Pool`          | gRPC connection pool driven by NodeResolver events    |
| `ClientMetrics` | Lightweight metrics snapshot (atomic counters)         |
| `PoolStats`     | Read-only pool state (total/healthy connections)       |

## Usage

### Create and start

```go
cfg := config.DefaultConfig()
cfg.Discovery.MDNSEnabled = true

client, err := client.NewLumenClient(cfg, logger)
if err != nil {
    log.Fatal(err)
}

ctx := context.Background()
if err := client.Start(ctx); err != nil {
    log.Fatal(err)
}
defer client.Close()
```

### Synchronous inference

```go
resp, err := client.Infer(ctx, &pb.InferRequest{
    Task:     "ocr",
    Data:     imageBytes,
    MimeType: "image/png",
})
```

### Streaming inference

```go
respChan, err := client.InferStream(ctx, req)
if err != nil {
    log.Fatal(err)
}
for resp := range respChan {
    fmt.Printf("Chunk: %s\n", string(resp.Result))
    if resp.IsFinal {
        break
    }
}
```

### Monitor nodes

```go
// List nodes
nodes := client.GetNodes()

// Watch for changes
client.WatchNodes(func(nodes []*discovery.NodeInfo) {
    fmt.Printf("Nodes updated: %d\n", len(nodes))
})
```

### Metrics

```go
metrics := client.GetMetrics()
fmt.Printf("Requests: %d, Success rate: %.1f%%\n",
    metrics.TotalRequests,
    (1 - metrics.ErrorRate) * 100)
```

## Discovery Backends

Discovery backends are additive, not prioritized: every configured backend
runs concurrently and their node events are merged. At least one must be
configured.

| Config                          | Backend          | Description                                             |
|----------------------------------|-------------------|----------------------------------------------------------|
| `Discovery.MDNSEnabled = true`   | `MDNSResolver`    | Zeroconf mDNS on local network                            |
| `Discovery.BrokerURL = "..."`    | `BrokerResolver`  | WebSocket push from a Lumen Host Broker                  |
| `Discovery.StaticNodes = [...]`  | `StaticResolver`  | Fixed `host:port` endpoints, no dynamic discovery          |


## Pool Behavior

- **NodeDiscovered** → caches resolved address candidates, dials gRPC, fetches capabilities, then marks ready
- **NodeExpired** → marks the discovery record stale but keeps an existing operational session unless removal is explicit
- **Explicit remove** → closes connection and removes the node from the pool
- **connectivity.Ready** → clears degradation state and moves to healthy subset
- **connectivity.TransientFailure/Shutdown** → enters temporary cooldown
- **Inference request/application errors** → do not affect node health
- **Inference connection errors** → count as hard failures; after 3 consecutive failures the node enters cooldown
- **Cooldown** → starts at 10s, doubles up to 2m, then the node may be picked again as a probe when no healthy node is available

## API Reference

| Method                | Description                          |
|-----------------------|--------------------------------------|
| `Start(ctx)`          | Start discovery and pool management  |
| `Close()`             | Stop discovery, close all connections|
| `Infer(ctx, req)`     | Synchronous inference                |
| `InferStream(ctx, req)` | Streaming inference                |
| `GetNodes()`          | List all pool connections            |
| `GetMetrics()`        | Get metrics snapshot                 |
| `PoolStats()`         | Get pool connection counts           |
| `WatchNodes(cb)`      | Register node change callback        |
| `GetConfig()`         | Get config copy                      |
