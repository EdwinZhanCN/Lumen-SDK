# Lumen SDK

Lumen SDK 是 Go 工具包，用于发现并调用分布式 Lumen ML 推理节点。

- `pkg/client`：gRPC 客户端、任务路由、连接池、健康状态和自动分块。
- `pkg/discovery`：mDNS、Host Broker WebSocket、静态节点发现。
- `pkg/hostbroker`：只提供节点发现控制面，不代理推理 Payload。
- `cmd/lumen-hostd`：跨平台 Host Broker 服务和 CLI。
- `pkg/types` / `proto`：统一任务、Tensor 和 gRPC 协议契约。

```go
cfg := config.DefaultConfig()
c, err := client.NewLumenClient(cfg, logger)
if err != nil { log.Fatal(err) }
if err := c.Start(ctx); err != nil { log.Fatal(err) }
defer c.Close()
resp, err := c.Infer(ctx, req)
```

构建和测试：`make build`、`go test ./...`。Host Broker 默认监听 `0.0.0.0:5866`，生产环境请配置网络隔离或认证层。
