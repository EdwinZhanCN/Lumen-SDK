# Lumen SDK

Lumen SDK 是 Lumilio/Lumen 体系中的 Go 数据平面客户端。它负责发现 Lumen Hub、建立 gRPC 连接、通过带内 capability RPC 验证协议兼容性，并按任务路由推理请求。

核心包：

- `pkg/client`：gRPC 客户端、连接池、协议验证、任务路由和分块传输。
- `pkg/discovery`：mDNS、静态地址及可选 Broker 的地址发现；发现结果不携带兼容性结论。
- `pkg/types` 与 `proto`：任务、Tensor 和 gRPC 数据平面契约。
- `pkg/hostbroker` / `cmd/lumen-hostd`：实验性的发现桥，仅在应用无法直接使用局域网发现时从源码构建。

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

## 实验性 Host Broker

Host Broker 是源码可用的高级工具，不是默认安装路径，也不发布预编译二进制。它只转发节点标识、地址和描述性元数据，不代理推理 payload，也不转发任务或协议权威信息。

```sh
make hostd-build
make hostd-run
```

SDK tag 用作源码版本和协议基线；它不承诺 `lumen-hostd` 的跨平台二进制发行。
