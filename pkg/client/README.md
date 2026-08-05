# Client Module

`pkg/client` composes discovery, one gRPC `ClientConn`, in-band capability validation, and task-aware routing.

## Authority model

Each fact has one owner:

```text
NodeResolver
  owns: identity and candidate endpoint
        │
        ▼
gRPC SubConn
  owns: transport availability
        │
        ▼
StreamCapabilities
  owns: protocol compatibility, services, models, and tasks
        │
        ▼
Picker
  routes only Ready + Compatible nodes that advertise the requested task
```

Discovery TXT values such as version and runtime are descriptive only. Task names, protocol versions, and capability hashes from discovery are intentionally ignored.

## Node state

`discovery.NodeInfo` is an immutable snapshot. It exposes two orthogonal states:

- `Availability`: `discovered`, `connecting`, `ready`, or `unavailable`; derived only from gRPC connectivity.
- `Compatibility`: `pending`, `compatible`, or `incompatible`; derived only from the capability stream.

A node is active and routable only when availability is `ready` and compatibility is `compatible`. A transport-ready but incompatible node remains visible for diagnostics and is never selected.

Capabilities are the canonical task representation. `SupportsTask`, `SupportsServiceTask`, and `MatchingServices` read the capability snapshot directly; there is no parallel task cache.

## Lifecycle

1. A discovery event adds or replaces an endpoint.
2. gRPC creates a `SubConn` and reports connectivity state.
3. Every genuine transition to `Ready` starts a generation-scoped capability fetch.
4. Temporary `Unavailable` responses are retried while the same endpoint generation remains ready.
5. An unimplemented capability RPC, missing/unparseable protocol version, or unsupported protocol major marks the node incompatible.
6. Endpoint replacement or transport reconnect starts a fresh validation generation. Results from older generations are discarded.
7. Resolver expiry removes the endpoint. Rediscovery creates a new operational session.

Inference/application errors do not affect transport health. Connection-level failures use a bounded cooldown before a node is probed again.

## Core API

| Type or method | Purpose |
|---|---|
| `LumenClient` | Starts discovery and performs inference |
| `Pool` | Owns the gRPC connection, resolver, balancer, and node registry |
| `PoolStats` | Reports `TotalNodes` and `RoutableNodes` |
| `GetNodes` / `NodeInfos` | Returns immutable node snapshots |
| `WatchNodes` / `OnNodesChanged` | Receives new immutable snapshots |
| `WithTask` | Adds the routing task to an RPC context |

## Example

```go
cfg := config.DefaultConfig()
c, err := client.NewLumenClient(cfg, logger)
if err != nil {
    return err
}
if err := c.Start(ctx); err != nil {
    return err
}
defer c.Close()

resp, err := c.Infer(ctx, &pb.InferRequest{
    Task:        "ocr",
    Payload:     imageBytes,
    PayloadMime: "image/png",
})
```

`Start` waits for at least one routable node. A node that is merely discovered, connecting, pending validation, or incompatible does not satisfy startup readiness.
