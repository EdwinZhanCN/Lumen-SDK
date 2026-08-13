# Lumen SDK

Lumen SDK 是 Lumilio/Lumen 体系中的 Go 数据平面客户端。它负责发现 Lumen Hub、建立 gRPC 连接、通过带内 capability RPC 验证协议兼容性，并按任务路由推理请求。

核心包：

- `pkg/client`：gRPC 客户端、连接池、协议验证、任务路由和分块传输。
- `pkg/discovery`：mDNS、静态地址及可选 Broker 的地址发现；发现结果不携带兼容性结论。
- `pkg/types` 与 `proto`：任务、Tensor 和 gRPC 数据平面契约。
- `pkg/hostbroker` / `cmd/lumen-hostd`：可选的发现兼容桥，仅用于应用无法直接使用局域网发现的场景。

## 运行边界

默认部署路径是应用直接通过 mDNS 或静态地址发现 Hub。发现层只回答“节点在哪里”；连接可用性来自 gRPC，协议与任务能力只来自 Hub 的 capability RPC。mDNS TXT 和 Host Broker 都不能声明节点兼容或可执行哪些任务。

```text
address discovery
      ↓
gRPC transport state
      ↓
StreamCapabilities
      ↓
protocol compatibility + task capabilities
      ↓
task-aware picker
```

```go
cfg := config.DefaultConfig()
c, err := client.NewLumenClient(cfg, logger)
if err != nil {
    log.Fatal(err)
}
if err := c.Start(ctx); err != nil {
    log.Fatal(err)
}
defer c.Close()

resp, err := c.Infer(ctx, req)
```

## 开发

```sh
make ci            # gofmt check + vet + race tests
make build         # compile SDK packages
make proto-check   # local proto lint/generated-code check
make proto-verify  # additionally compare with the pinned remote wire baseline
```

## 可选 Host Broker

Host Broker 不是默认或推荐的发现路径；应用应优先直接使用 mDNS 或静态地址。对于 Docker Desktop 等无法直接执行局域网发现的环境，可以将它作为兼容桥。它只转发节点标识、地址和描述性元数据，不代理推理 payload，也不转发任务或协议权威信息。

Release tag 会继续发布 Linux、macOS 和 Windows 的预编译 `lumen-hostd` 产物；也可以从源码构建：

```sh
make hostd-build
make hostd-run
```

维护者可以在发 tag 前本地验证完整发布产物：

```sh
make release VERSION=vX.Y.Z
make tag VERSION=vX.Y.Z # push tag 并触发 GitHub Release
```

SDK tag 同时用作源码版本、协议基线和对应 `lumen-hostd` 二进制发行版本。
