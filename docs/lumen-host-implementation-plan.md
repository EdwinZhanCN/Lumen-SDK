# Exec Plan: Refactor Lumen Gateway into a Cross-Platform Host Discovery Broker

**Status:** Proposed
**Primary repository:** `EdwinZhanCN/Lumen-SDK`
**Consumer repository:** `EdwinZhanCN/Lumilio-Photos`
**Working component name:** `Lumen Host`
**Working daemon name:** `lumen-hostd`
**Compatibility name:** `lumengatewayd`
**Estimated rollout:** Multiple independently shippable milestones

---

## 1. Purpose

Refactor the existing Lumen Gateway daemon into a small, headless host service responsible for discovering Lumen inference nodes from the host operating system and publishing normalized node events to applications that cannot perform reliable local-network discovery themselves.

The immediate target is Lumilio Photos running inside Docker Desktop on macOS or Windows:

```text
Lumilio container
    │
    │ ordinary TCP/WebSocket connection
    ▼
Lumen Host running on the host OS
    │
    │ mDNS on physical network interfaces
    ▼
Lumen inference nodes on the LAN
```

The Host service is a **control-plane broker only**.

It discovers nodes and reports their endpoints and capabilities. It does not proxy image data, tensors, embeddings, or inference requests during the first implementation.

---

## 2. Problem statement

Lumilio can discover Lumen nodes directly through mDNS when it shares the host network namespace. This works naturally with Linux host networking, but Docker Desktop on macOS and Windows places containers behind a VM-backed bridge network where LAN multicast discovery is not consistently available.

Lumilio already documents three discovery modes:

1. Linux host-network mDNS;
2. Gateway push through `LUMEN_DISCOVERY_HUB_URL`;
3. Explicit static node addresses.

The release Compose configuration also already maps `host.docker.internal` through `host-gateway`, while the Linux-specific overlay enables host networking and mDNS. citeturn637970view1turn815724view0turn815724view1

The existing Gateway solves much of this problem, but currently mixes several responsibilities:

- host-level discovery;
- Lumen SDK client lifecycle;
- node connection management;
- REST inference proxying;
- metrics;
- CLI inference;
- daemon lifecycle management;
- Wails system-tray UI.

This makes the Gateway appear to be a second end-user application rather than infrastructure owned by Lumilio.

---

## 3. Existing implementation map

### 3.1 Discovery abstraction

`pkg/discovery/resolver.go` defines `NodeResolver` as an event source:

```go
type NodeResolver interface {
    Watch(ctx context.Context) (<-chan NodeEvent, error)
}
```

The existing event model includes discovery, expiry and resolution failures. Discovery is intentionally distinct from operational liveness; the client connection pool owns connection health. citeturn822567view0

`pkg/discovery/composite_resolver.go` already merges multiple discovery providers into one event stream. Current providers include:

- `MDNSResolver`;
- `PushResolver`;
- `StaticResolver`.

This means a Host Broker can be introduced without changing the SDK’s fundamental discovery model. citeturn822567view2

### 3.2 Existing remote discovery protocol

`pkg/discovery/push_resolver.go` already connects to:

```text
/v1/nodes/watch
```

over WebSocket, receives an initial snapshot, handles added and removed node events, and reconnects with exponential backoff. citeturn822567view1

`pkg/server/rest/node_watch.go` already implements the server side of that protocol. A newly connected client receives a node snapshot and subsequently receives node changes. The current wire descriptor includes node identity, address, supported tasks and TXT metadata. citeturn624716view0

Therefore, the Host Broker protocol does not need to be designed from scratch.

### 3.3 Existing daemon

`cmd/lumengatewayd/service/gatewayd.go` currently:

1. creates a complete `LumenClient`;
2. starts its discovery and connection pool;
3. starts the REST router;
4. exposes node watching as well as inference-related endpoints. citeturn697426view0


`cmd/lumengatewayd/main.go` also implements its own process detachment and PID-file management instead of relying on launchd, systemd or Windows service management. citeturn634644view0

### 3.4 Existing tray application

`cmd/lumen-gateway` creates a Wails application and system tray. Its `GatewayService` duplicates daemon responsibilities by creating a Lumen client, starting the REST server, exposing node and metrics state and owning application lifecycle. citeturn166261view1turn166261view3

### 3.5 Existing configuration

`DiscoveryConfig` currently exposes:

```go
MDNSEnabled bool
HubURL      string
StaticNodes []string
```

The three sources are additive, and `NewLumenClient` constructs the corresponding resolvers. The current environment variable is:

```text
LUMEN_DISCOVERY_HUB_URL
```

The default configuration enables mDNS and leaves `HubURL` empty. citeturn521701view0turn521701view1

---

## 4. Core architectural decision

The refactor will introduce the following ownership model:

```text
Lumen Host
    Owns:
    - access to host network interfaces
    - mDNS discovery
    - future Matter commissioning/discovery
    - a normalized node registry
    - discovery event publication
    - limited diagnostics

Lumen SDK
    Owns:
    - combining discovery sources
    - selecting nodes by capability
    - gRPC connection management
    - operational health
    - task-aware routing
    - retry and cooldown
    - inference transport

Lumen Node
    Owns:
    - model lifecycle
    - capability advertisement
    - inference execution

Lumilio
    Owns:
    - user-facing configuration
    - displaying node status
    - deciding which ML tasks to perform
```

The essential division is:

```text
Host Broker: Who exists?
Lumen SDK:   Which node should handle this request?
Lumen Node:  Execute the request.
Lumilio:     Why is the request needed?
```

---

## 5. Non-goals

The first version will not:

- proxy inference payloads through the Broker;
- replace the existing Lumen inference gRPC protocol;
- implement Matter;
- implement a complete certificate authority;
- provide third-party application authorization;
- provide remote Internet discovery;
- provide cluster-wide scheduling;
- expose a public plugin ecosystem;
- require a GUI;
- require a new distributed database;
- guarantee access to IPv6 link-local-only nodes from Docker Desktop;
- remove all existing Gateway names in the first release.

Matter will remain a future discovery and trust provider behind the same Broker interface.

---

## 6. Required invariants

The implementation must preserve these properties.

### 6.1 Discovery is not liveness

A node announced by mDNS is a discovery candidate. Its actual availability must continue to be determined by the SDK’s gRPC connection state and capability probing.

### 6.2 The Broker does not enter the inference data path

Normal inference remains:

```text
Lumilio container
        │
        └──────────── gRPC ────────────► Lumen Node
```

not:

```text
Lumilio → Broker → Node → Broker → Lumilio
```

### 6.3 Broker failure does not break media management

If `lumen-hostd` stops:

- Lumilio remains available;
- ordinary photo and video management remains available;
- existing direct node connections may continue until disconnected;
- ML features degrade to unavailable;
- the SDK continues reconnecting without crashing the application.

### 6.4 Existing discovery modes remain additive

Static nodes, direct mDNS and Broker discovery can coexist.

### 6.5 Existing installations receive a compatibility window

`HubURL`, `LUMEN_DISCOVERY_HUB_URL`, `/v1/nodes/watch` and the `lumengatewayd` binary name must remain functional for at least one release cycle after their replacements are introduced.

---

# 7. Target architecture

```text
┌──────────────── Host operating system ────────────────┐
│                                                       │
│  lumen-hostd                                          │
│  ├── MDNSDiscoveryProvider                            │
│  ├── future MatterDiscoveryProvider                   │
│  ├── NodeRegistry                                     │
│  ├── Host interface monitoring                        │
│  ├── Broker HTTP/WebSocket API                        │
│  └── local authentication                             │
│                                                       │
└──────────────────────────┬────────────────────────────┘
                           │
                 host.docker.internal:5866
                           │
┌──────────────── Docker environment ───────────────────┐
│                                                       │
│  Lumilio server                                       │
│      │                                                │
│      ▼                                                │
│  Lumen SDK                                            │
│  ├── BrokerResolver                                   │
│  ├── StaticResolver                                   │
│  ├── optional direct MDNSResolver                     │
│  ├── CompositeResolver                                │
│  ├── connection pool                                  │
│  └── task-aware balancer                              │
│                                                       │
└──────────────────────────┬────────────────────────────┘
                           │ direct gRPC
                           ▼
                   LAN Lumen Nodes
```

---

# 8. Milestone 0 — Characterization and safety tests

## Goal

Lock down current behavior before renaming or removing functionality.

## Work

Add characterization tests for:

- the current `/v1/nodes/watch` snapshot;
- node-added events;
- node-removed events;
- PushResolver reconnect behavior;
- duplicate events;
- CompositeResolver source merging;
- daemon shutdown;
- SDK behavior when the Gateway is unavailable.

Use fake `NodeResolver` implementations instead of requiring mDNS in ordinary unit tests.

## Target files

```text
pkg/discovery/push_resolver_test.go
pkg/discovery/composite_resolver_test.go
pkg/server/rest/node_watch_test.go
cmd/lumengatewayd/service/gatewayd_test.go
```

## Acceptance criteria

- Existing event format is covered by tests.
- Reconnection does not create duplicate active nodes.
- A disconnected watch client does not block event publication.
- Closing the daemon closes watchers without leaking goroutines.
- No behavior change is shipped in this milestone.

---

# 9. Milestone 1 — Introduce Broker terminology without breaking compatibility

## Goal

Rename the conceptual API while preserving current public behavior.

## Configuration changes

Extend `pkg/config.DiscoveryConfig`:

```go
type DiscoveryConfig struct {
    // Existing fields...

    BrokerURL string `yaml:"broker_url" json:"broker_url"`

    // Deprecated compatibility field.
    HubURL string `yaml:"hub_url" json:"hub_url"`
}
```

Add:

```text
LUMEN_DISCOVERY_BROKER_URL
```

Resolution order:

```text
1. BrokerURL / LUMEN_DISCOVERY_BROKER_URL
2. HubURL / LUMEN_DISCOVERY_HUB_URL
3. no Broker resolver
```

If both new and deprecated values are present and differ, configuration validation must return an error rather than silently selecting one.

## Resolver naming

Introduce:

```go
type BrokerResolver struct {
    // existing PushResolver implementation
}
```

Compatibility options:

```go
type PushResolver = BrokerResolver
```

or:

```go
func NewPushResolver(...) *BrokerResolver {
    return NewBrokerResolver(...)
}
```

Do not maintain two independent implementations.

## Target files

```text
pkg/config/config.go
pkg/config/defaults.go
pkg/config/config_test.go

pkg/discovery/push_resolver.go
pkg/discovery/broker_resolver.go
pkg/discovery/broker_resolver_test.go

pkg/client/client.go
```

## Acceptance criteria

- New applications can configure `BrokerURL`.
- Existing `HubURL` configurations continue working.
- Only one Broker WebSocket connection is opened.
- SDK logs refer to “broker discovery,” not “gateway push.”
- Deprecated names are clearly marked in GoDoc and configuration documentation.

---

# 10. Milestone 2 — Extract a discovery-only Host Broker mode

## Goal

Make the daemon serve node discovery without serving inference.

## Initial implementation strategy

Do not immediately rewrite the node registry.

For the first implementation, continue using `LumenClient` internally because it already provides:

- mDNS discovery;
- node identity normalization;
- capability fetching;
- node watches;
- connection state integration.

However, construct the internal client with strict Broker configuration:

```text
MDNS enabled:     true
Static nodes:     optional
Broker URL:       empty
REST inference:   disabled
```

The empty Broker URL is important to avoid a daemon subscribing back to itself.

## Broker server surface

The discovery-only server should expose:

```text
GET /v1/health
GET /v1/version
GET /v1/nodes
GET /v1/nodes/watch
```

During the compatibility period, retain the existing endpoint path.

Do not register:

```text
POST /v1/infer
inference streaming endpoints
LLM tool endpoints
MCP endpoints
```

## Internal server separation

Extract route construction so the daemon does not import the complete REST inference router.

Proposed layout:

```text
pkg/hostbroker/
├── server.go
├── routes.go
├── node_watch.go
├── wire.go
├── auth.go
└── server_test.go
```

Avoid making `pkg/hostbroker` depend directly on the concrete `*client.LumenClient`. Define a minimal interface:

```go
type NodeCatalog interface {
    GetNodes() []client.NodeInfo
    WatchNodes(callback func(client.NodeEvent)) (cancel func())
}
```

The existing Lumen client can implement or be adapted to this interface.

This keeps the future door open for a lighter discovery-only catalog without requiring that rewrite now.

## Daemon changes

Refactor:

```text
cmd/lumengatewayd/service/gatewayd.go
```

into:

```text
cmd/lumen-hostd/service/hostd.go
```

**Revised (2026-07-10):** the plan originally suggested compiling one
implementation into two binary names (`lumen-hostd` and `lumengatewayd`,
with the latter printing a deprecation notice) for compatibility. Since
`cmd/lumengatewayd` is not used in any production environment, this rollout
renames it to `cmd/lumen-hostd` outright instead — no `lumengatewayd` binary
name survives, and there's no deprecation-notice shim to build or maintain.

## Acceptance criteria

- The daemon discovers mDNS nodes and publishes them to a Broker client.
- The daemon does not expose inference endpoints.
- The Broker process never receives media payloads.
- An SDK using `BrokerResolver` can connect directly to a discovered node.
- Existing `/v1/nodes/watch` clients continue working.
- The daemon does not subscribe to itself.

---

# 11. Milestone 3 — Stabilize the Broker wire protocol

## Goal

Make the discovery protocol explicit and versionable.

## Current event model

Preserve the existing event types during migration:

```json
{
  "type": "snapshot",
  "nodes": []
}
```

```json
{
  "type": "added",
  "node": {}
}
```

```json
{
  "type": "removed",
  "node_id": "..."
}
```

Add a protocol-version field to the initial snapshot:

```json
{
  "type": "snapshot",
  "protocol_version": 1,
  "broker_version": "1.4.0",
  "sequence": 42,
  "nodes": []
}
```

## Node descriptor

Define one canonical wire structure:

```go
type BrokerNode struct {
    NodeID       string            `json:"node_id"`
    DeploymentID string            `json:"deployment_id,omitempty"`
    Endpoints    []string          `json:"endpoints"`
    Tasks        []string          `json:"tasks"`
    TXT          map[string]string `json:"txt,omitempty"`
    Source       string            `json:"source"`
    LastSeen     time.Time         `json:"last_seen"`
}
```

Prefer plural `Endpoints` even if the first version usually contains one address.

The Broker should report endpoints; it should not claim that every endpoint is reachable from every Docker environment.

## Update events

Introduce:

```text
updated
```

for capability or address changes.

For compatibility, old clients may continue interpreting an update as:

```text
removed + added
```

## Ordering and recovery

Every event receives a monotonically increasing process-local sequence number.

When a client reconnects, it receives a complete snapshot. The first implementation does not need durable event replay.

## Acceptance criteria

- New clients reject unsupported major protocol versions.
- Old clients continue understanding snapshot, added and removed.
- Reconnection always reconstructs the complete node state.
- Node updates do not produce permanently duplicated entries.
- Node IDs are stable across address changes.

---

# 12. Milestone 4 — Add minimum viable authentication

**Status: skipped for the initial rollout (2026-07-10)** — see §23 PR 5.
The Broker API ships without bearer-token auth in this rollout; anyone on
the LAN who can reach the Broker's port can read node topology. Revisit
before recommending the Broker for untrusted or shared networks.

## Goal

Prevent arbitrary LAN clients from reading Broker state while avoiding a full PKI project.

## Threat boundary

The Broker API is read-only and does not carry inference data, but it still reveals:

- node identities;
- network addresses;
- available capabilities;
- local infrastructure topology.

It should therefore not be intentionally exposed without authentication.

## Token model

On first installation, generate a random 256-bit token:

```text
macOS:
~/Library/Application Support/Lumen/host-token

Windows:
%LOCALAPPDATA%\Lumen\host-token

Linux user service:
~/.config/lumen/host-token

Linux system service:
/var/lib/lumen/host-token
```

Use restrictive file permissions where supported.

Clients send:

```text
Authorization: Bearer <token>
```

The WebSocket handshake must validate the header before upgrading.

## SDK configuration

Add:

```go
BrokerToken     string
BrokerTokenFile string
```

Prefer token files over environment-variable secrets.

Environment variables:

```text
LUMEN_DISCOVERY_BROKER_TOKEN
LUMEN_DISCOVERY_BROKER_TOKEN_FILE
```

## Docker integration

Mount the token read-only:

```yaml
services:
  server:
    environment:
      LUMEN_DISCOVERY_BROKER_URL: http://host.docker.internal:5866
      LUMEN_DISCOVERY_BROKER_TOKEN_FILE: /run/secrets/lumen-host-token

    volumes:
      - ${LUMEN_HOST_TOKEN_PATH}:/run/secrets/lumen-host-token:ro

    extra_hosts:
      - "host.docker.internal:host-gateway"
```

## Bind policy

Preferred order:

1. bind to a host address reachable through `host.docker.internal`;
2. require the bearer token;
3. expose only discovery routes;
4. document firewall behavior.

Do not assume that binding only to loopback behaves identically across every Docker Desktop version. Test it on supported macOS and Windows versions before selecting the final default.

## Non-goal

This token does not claim to provide Matter-level device identity or protection against a fully compromised host. It is a containment measure for the current Lumilio-only deployment.

## Acceptance criteria

- Requests without a token receive `401`.
- Invalid tokens receive `401` without revealing node data.
- WebSocket upgrades require authentication.
- Token values never appear in logs.
- Lumilio can read the token from a mounted file.
- Token rotation is possible by restarting the Broker and client.

---

# 13. Milestone 5 — Replace custom daemonization with native service management

## Goal

Run Lumen Host as an invisible background service without a tray application.

The process itself should run in the foreground. The operating system service manager owns restart, logging and lifecycle.

## Remove from daemon core

Remove or deprecate:

- manual `--daemon` process detachment;
- PID files in `/tmp`;
- self-managed background spawning;
- platform-specific detached-process code.

Keep an optional foreground command:

```text
lumen-hostd serve
```

This is useful for development, containers and diagnostics.

---

## 13.1 macOS

Use a per-user LaunchAgent initially.

Proposed files:

```text
packaging/macos/com.edwinzhan.lumen-host.plist
scripts/install-macos.sh
scripts/uninstall-macos.sh
```

Installation locations:

```text
~/Library/Application Support/Lumen/lumen-hostd
~/Library/LaunchAgents/com.edwinzhan.lumen-host.plist
```

LaunchAgent properties:

- `RunAtLoad`;
- `KeepAlive`;
- standard output and error log paths;
- no Dock icon;
- no menu-bar icon;
- no root requirement for the default installation.

Use a LaunchDaemon only if a future requirement demands operation before user login.

---

## 13.2 Windows

Implement in two stages.

### Initial mode

Install a hidden per-user process through Task Scheduler:

- starts at user login;
- automatically restarts;
- requires no administrator permission;
- writes logs under `%LOCALAPPDATA%\Lumen`.

### Future system-service mode

Add a Windows Service only when operation without a logged-in user becomes a real requirement.

Do not block the Host Broker refactor on implementing both modes.

---

## 13.3 Linux

Provide:

```text
packaging/linux/lumen-host.service
packaging/linux/lumen-host-user.service
```

Support:

- systemd user service for desktop users;
- system systemd service for servers;
- foreground process for containers;
- optional host-network sidecar.

Example sidecar topology:

```text
lumen-host container
    network_mode: host

Lumilio container
    bridge network
    connects through host-gateway:5866
```

## Acceptance criteria

- Closing a terminal does not stop the service.
- No tray icon or GUI is required.
- Service logs are available through native platform tooling.
- Crashed services restart automatically.
- Uninstall removes service definitions without deleting unrelated Lumilio data.

---

# 14. Milestone 6 — Integrate Broker discovery into Lumilio deployment

**Status: skipped for the initial rollout (2026-07-10)** — see §23 PR 6.
Lumilio-Photos (separate repo) is not updated to consume Broker discovery in
this rollout; direct mDNS, static nodes, and `LUMEN_DISCOVERY_HUB_URL` remain
the only ways Lumilio finds nodes. This also means Milestone 7's "move node
UI into Lumilio" has no destination yet — see the note there.

## Goal

Make Host Broker discovery a first-class Lumilio configuration without making it mandatory.

## Compose changes

Retain the existing:

```yaml
extra_hosts:
  - "host.docker.internal:host-gateway"
```

Add documented variables:

```text
LUMEN_DISCOVERY_BROKER_URL
LUMEN_DISCOVERY_BROKER_TOKEN_FILE
```

Continue accepting:

```text
LUMEN_DISCOVERY_HUB_URL
```

during the compatibility period.

Do not immediately hardcode the Broker URL as enabled for every installation. First add detection and UI so a missing Broker does not produce confusing permanent reconnect logs.

## Suggested discovery modes

Expose an application-level setting:

```text
Automatic
Direct mDNS
Host Broker
Static nodes
Disabled
```

`Automatic` resolution:

```text
1. Use configured Broker when reachable.
2. Use direct mDNS when enabled and supported.
3. Merge configured static nodes.
4. Continue without ML when none are available.
```

The actual SDK providers remain additive even if the UI presents a simplified mode.

## Lumilio settings UI

Move relevant information from the Gateway tray application into:

```text
Settings
└── AI & Inference
    ├── Host Broker status
    ├── discovered nodes
    ├── node capabilities
    ├── connection state
    ├── last discovery error
    └── setup instructions
```

Do not expose low-level connection-pool implementation details by default.

## First-run behavior

When Lumilio is running in Docker and no nodes are available:

```text
No local AI provider was found.

Install Lumen Host to automatically discover inference nodes
on this computer's local network, or configure a node address manually.
```

The absence of Lumen Host must not block application initialization.

## Acceptance criteria

- Docker Desktop users do not enter inference-node IP addresses manually.
- Linux users can still use direct host-network mDNS.
- Static nodes continue working.
- The application clearly distinguishes:
  - Broker unavailable;
  - Broker available but no nodes;
  - nodes discovered but unreachable;
  - node connected and ready.

---

# 15. Milestone 7 — Retire the Gateway GUI and broad CLI

## Goal

Eliminate the second end-user application after equivalent observability exists in Lumilio.

## Tray application

Remove `cmd/lumen-gateway` only after:

- Lumen Host has reliable service installation;
- Lumilio displays discovered nodes;
- diagnostics remain available;
- configuration migration has shipped.

Do not delete the GUI at the same time as the first Broker refactor. That would combine architectural, deployment and user-interface risk in one release.

**Scope note (2026-07-10, revised):** Milestone 6 is skipped in this
rollout, so "Lumilio displays discovered nodes" will not be true when this
milestone ships — normally a reason to deprecate rather than delete. The
user confirmed `cmd/lumen-gateway` is not used in any production
environment, though, so §23's combined PR 3 deletes it (and
`cmd/lumengateway`) outright rather than deprecating first. "Move node UI
into Lumilio" is simply dropped from scope, not carried forward.

## CLI

Remove or deprecate inference-facing commands such as:

```text
infer
task execution
generic REST inference
```

Keep a small administrative surface:

```text
lumen-hostd serve
lumen-hostd install
lumen-hostd uninstall
lumen-hostd start
lumen-hostd stop
lumen-hostd status
lumen-hostd doctor
lumen-hostd version
```

A separate `lumen-hostctl` binary is optional. Keeping these commands in the daemon binary reduces packaging work.

## `doctor` output

`doctor` should test:

1. service running;
2. Broker port reachable;
3. authentication token readable;
4. physical network interfaces detected;
5. mDNS browser started;
6. discovered node count;
7. node endpoint formatting;
8. Docker host name guidance;
9. direct gRPC reachability for each node.

It must not upload logs or data.

## Acceptance criteria

- Normal users never need to launch a Gateway application.
- Lumilio owns all user-facing node configuration.
- Maintainers retain enough diagnostics to investigate installation problems.
- The inference protocol remains testable through SDK examples or dedicated development tools.

---

# 16. Milestone 8 — Optional internal cleanup

This milestone is not required for the first Host Broker release.

Once behavior is stable, replace the Broker’s internal full `LumenClient` with a lighter node catalog if the current client causes unnecessary connections or resource use.

Possible structure:

```text
pkg/nodecatalog/
├── catalog.go
├── reducer.go
├── capabilities.go
└── watch.go
```

The catalog consumes `NodeResolver` events and owns:

- node identity merging;
- address updates;
- source tracking;
- capability cache;
- snapshots;
- watchers.

Operational gRPC health and task balancing would remain SDK-only.

Only perform this extraction when profiling or maintenance experience shows that using the full client inside `lumen-hostd` is problematic.

---

# 17. Matter integration boundary

Matter is explicitly deferred, but the Host Broker must allow a future provider:

```go
type DiscoveryProvider interface {
    Watch(ctx context.Context) (<-chan NodeEvent, error)
}
```

Future structure:

```text
lumen-hostd
├── MDNSResolver
├── StaticResolver
└── MatterResolver
```

Matter-discovered and mDNS-discovered representations must converge into the same node registry.

No Matter-specific concepts should leak into:

- the Lumen inference gRPC API;
- Lumilio business logic;
- the connection balancer;
- the photo-processing pipeline.

The future Matter provider may add trust metadata, but node discovery and inference transport remain separate concerns.

---

# 18. Testing strategy

## 18.1 Unit tests

### Discovery

- Broker snapshot reduction;
- node add/update/remove;
- duplicate node IDs from different providers;
- address changes;
- capability changes;
- source disappearance;
- CompositeResolver cancellation;
- reconnect backoff;
- context cancellation;
- slow watch client isolation.

### Authentication

- valid bearer token;
- missing token;
- malformed header;
- incorrect token;
- token file not found;
- token file unreadable;
- token rotation;
- no token content in logs.

### Configuration

- `BrokerURL` only;
- deprecated `HubURL` only;
- both equal;
- both conflicting;
- environment overrides;
- empty discovery configuration;
- static node fallback.

---

## 18.2 In-process integration tests

Construct:

```text
Fake NodeResolver
      ↓
Host Broker test server
      ↓
BrokerResolver
      ↓
CompositeResolver
      ↓
LumenClient test pool
```

Verify that:

- a fake node becomes visible to the client;
- capability changes propagate;
- removal reaches the client;
- Broker restart causes reconnection;
- client restart receives a complete snapshot;
- multiple clients receive independent streams.

---

## 18.3 Docker integration tests

### Linux

Test:

- Lumilio bridge network;
- Host Broker on host;
- Host Broker host-network container;
- `host.docker.internal` with `host-gateway`;
- direct data-plane connection to a fake Lumen node.

### macOS Docker Desktop

Test:

- Broker installed as LaunchAgent;
- Lumilio container reaches Broker;
- Broker receives LAN mDNS;
- container receives node events;
- container directly reaches the advertised node;
- switching Wi-Fi networks;
- sleep and resume;
- Docker Desktop restart;
- Broker restart.

### Windows Docker Desktop

Test:

- per-user background process;
- Windows Firewall prompt behavior;
- WSL2/Docker Desktop network path;
- Wi-Fi reconnect;
- sleep and resume;
- service restart;
- container-to-Broker authentication.

---

## 18.4 Physical-network smoke tests

Use at least:

- one macOS host;
- one Windows host;
- one Linux inference node;
- one second subnet or guest Wi-Fi test;
- multiple active network interfaces;
- VPN enabled and disabled.

mDNS tests must specify which interfaces are expected to participate.

---

# 19. Critical technical risks

## 19.1 Discovered address may not be reachable from the container

The Broker may discover:

- IPv6 link-local addresses;
- interface-scoped addresses;
- hostnames only resolvable by the host;
- addresses blocked from Docker’s VM.

Mitigation:

- include all candidate endpoints;
- prioritize routable IPv4 addresses initially;
- preserve endpoint source metadata;
- let the SDK probe candidates;
- explicitly mark link-local-only nodes unsupported in the first Docker release;
- add an optional data proxy only if direct reachability proves to be a frequent real-world failure.

Do not introduce inference proxying preemptively.

## 19.2 Multiple network interfaces

The host may have:

- Wi-Fi;
- Ethernet;
- VPN;
- virtual adapters;
- Docker interfaces.

The Broker must avoid advertising or browsing on obviously irrelevant interfaces while still reacting when the preferred physical interface changes.

The first version may use conservative platform defaults and expose interface information through `doctor`.

## 19.3 Firewall prompts

A background process accepting local TCP connections may trigger host firewall behavior.

Mitigation:

- stable binary path;
- stable signing identity when available;
- fixed default port;
- clear installation documentation;
- avoid random listening ports;
- test signed and unsigned development builds separately.

## 19.4 Token distribution friction

A mounted token file adds setup complexity.

Mitigation:

- installer prints or writes the Compose environment path;
- Lumilio setup documentation detects standard token locations;
- future desktop installers can configure the mount automatically;
- static-node mode remains available.

## 19.5 Code-signing remains unsolved

Removing Wails reduces application complexity but does not remove macOS Gatekeeper or Windows SmartScreen requirements.

Unsigned daemon distribution remains suitable for development and technically experienced users, but a polished public installation eventually requires platform signing and, on macOS, notarization.

This is a release-management issue, not a reason to retain the tray GUI.

---

# 20. Compatibility and migration policy

## Release N

Introduce:

- `BrokerURL`;
- `BrokerResolver`;
- discovery-only daemon mode;
- `lumen-hostd` name;
- old names remain fully supported.

## Release N+1

Make Host Broker the documented recommendation for Docker Desktop.

- Tray GUI becomes deprecated.
- Inference CLI becomes deprecated.
- Lumilio settings display Broker state.

## Release N+2 or next major version

Remove:

- Gateway inference REST routes;
- Wails tray application;
- broad inference CLI;
- deprecated `HubURL` naming, subject to actual downstream usage.

Retain a compatibility executable or migration error message where inexpensive.

---

# 21. Rollback strategy

Each milestone must be independently reversible.

### Broker terminology rollback

Continue using `HubURL` and `PushResolver`.

### Discovery-only server rollback

Re-enable the current REST router without changing SDK discovery.

### Native-service rollback

Run `lumen-hostd serve` manually or restore current daemonization.

### Lumilio integration rollback

Return to:

- direct mDNS on Linux;
- static nodes on Docker Desktop;
- old Gateway URL.

No database migration is required for the first Host Broker implementation, so rollback should not affect media-library state.

---

# 22. Definition of done

**Note (2026-07-10):** items 4, 6, and 11 below depend on Milestone 6
(Lumilio Docker integration) and Milestone 4 (token authentication), both
skipped for the initial rollout (§23). They remain the eventual target, not
current scope.

The Host Broker project is complete when all of the following are true:

1. A macOS user can install one background service with no tray application.
2. A Windows user can install one background process with no persistent UI.
3. A Linux user can use either a systemd service or host-network container.
4. Lumilio running in Docker connects to the Broker through `host.docker.internal`.
5. The Broker discovers LAN Lumen nodes through host mDNS.
6. Lumilio receives node snapshots and changes without manual IP entry.
7. Inference traffic travels directly from Lumilio to the selected Lumen node.
8. Broker failure disables discovery but does not crash Lumilio.
9. Missing Broker configuration does not prevent Lumilio startup.
10. Static-node and Linux direct-mDNS modes remain functional.
11. The Broker API requires the configured token.
12. The tray GUI is no longer necessary for ordinary operation.
13. Maintainers can run `doctor` and determine whether failure is in:
    - service installation;
    - authentication;
    - discovery;
    - address reachability;
    - node capability;
    - inference connection.
14. Existing Gateway configurations receive a documented migration path.

---

# 23. Recommended implementation order

The first pull requests should be deliberately small.

**Rollout scope decision (2026-07-10, revised):** PR 5 (token authentication)
and PR 6 (Lumilio Docker integration) are skipped for this rollout. PR 3,
PR 4, PR 7, **and PR 8** are squashed into one combined PR. PR 8 was
initially kept separate because §15 warns against combining architectural,
deployment, and UI-removal risk in one release — but the user confirmed
neither `cmd/lumen-gateway` (Wails tray) nor `cmd/lumengateway` (CLI) is used
in any production environment, so that risk doesn't apply here and a fully
destructive migration is preferred over a deprecate-first approach. This also
means `cmd/lumengatewayd` is renamed to `cmd/lumen-hostd` outright — no
compatibility binary under the old name, unlike the plan's original
Milestone 2 design. A destructive removal of the old binaries has a larger
blast radius than just their two `cmd/` directories: the root `Makefile`,
both CI workflows, `README.md`, and `docs/configuration.md`,
`docs/development.md`, `docs/installation.md` all build, test, or document
the old `lumengatewayd`/`lumengateway`/Wails-app names and need to change in
the same PR so nothing is left broken or stale. Mapping from the original
numbering:

| Original PR | Status |
|---|---|
| PR 1 — Characterization tests | Done (commit `7b6d896`) |
| PR 2 — Broker naming and configuration aliases | Done (commit `da9c93a`) |
| PR 3 — Discovery-only server package | Squashed into combined PR 3, below |
| PR 4 — `lumen-hostd` binary | Squashed into combined PR 3, below (full rename, no compat binary) |
| PR 5 — Token authentication | Skipped (§12) |
| PR 6 — Lumilio Docker integration | Skipped (§14) |
| PR 7 — Native service packaging | Squashed into combined PR 3, below |
| PR 8 — UI and CLI retirement | Squashed into combined PR 3, below (destructive, not deprecate-first) |
| PR 9 — Matter exploration | Unchanged, still separate/optional |

## PR 1 — Characterization tests

```text
Test current PushResolver and node-watch behavior.
No production behavior changes.
```

Done — commit `7b6d896`.

## PR 2 — Broker naming and configuration aliases

```text
BrokerURL
BrokerResolver
deprecated HubURL compatibility
```

Done — commit `da9c93a`.

## PR 3 (combined) — Host Broker binary, discovery-only server, native packaging, and old-app removal

```text
pkg/hostbroker: health/version/nodes/watch, no inference routes           [was PR 3]
cmd/lumengatewayd renamed to cmd/lumen-hostd (foreground process,
    no compat binary under the old name)                                  [was PR 4]
Native service packaging: macOS LaunchAgent, Windows user startup,
    Linux systemd units, doctor command                                   [was PR 7]
Delete cmd/lumen-gateway (Wails tray) and cmd/lumengateway (CLI) outright  [was PR 8]
Update Makefile, ci.yml, release.yml, README.md, docs/configuration.md,
    docs/development.md, docs/installation.md to match
```

"Move node UI into Lumilio" (part of the original PR 8) still has no
destination since PR 6 is skipped — that line item is dropped, not done.

## ~~PR 5 — Token authentication~~ (skipped)

```text
token generation
WebSocket authorization
SDK token-file support
```

Skipped for this rollout — see §12.

## ~~PR 6 — Lumilio Docker integration~~ (skipped)

```text
Broker configuration
host.docker.internal
settings status
graceful unavailable state
```

Skipped for this rollout — see §14.

## PR 9 — Matter exploration

```text
separate PoC
no production dependency
```

---

# 24. First concrete coding task

The safest first production change is:

> Add `BrokerURL` as an alias for the existing `HubURL`, rename `PushResolver` conceptually to `BrokerResolver`, and prove through tests that both names use the same existing `/v1/nodes/watch` implementation.

This establishes the final vocabulary without changing networking, daemon lifecycle or user deployment.

After that, extract the discovery-only route set from the current REST server.

Do not begin with native installers or Matter.
