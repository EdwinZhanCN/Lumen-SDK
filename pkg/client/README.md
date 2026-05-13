# Client Module

## 概述

Client模块是Lumen SDK的核心组件，提供分布式推理服务的客户端实现。它负责服务发现、负载均衡、连接管理和推理调用，为上层应用提供统一、高性能的AI服务访问接口。

## 主要功能

### 🔍 服务发现 (Service Discovery)
- 基于mDNS的自动节点发现
- 实时节点状态监控
- 动态节点列表更新
- 节点健康检查

### ⚖️ 负载均衡 (Load Balancing)
- 多种负载均衡策略：轮询、随机、最少负载
- 智能节点选择算法
- 节点容量感知调度
- 故障节点自动剔除

### 🔗 连接池管理 (Connection Pool)
- gRPC连接复用
- 连接生命周期管理
- 自动连接重建
- 连接数限制和优化

### 📊 监控指标 (Metrics)
- 请求成功率和延迟统计
- 节点负载和资源使用
- 实时性能指标
- 可观测性支持

## 核心组件

### LumenClient
主客户端类，提供统一的AI服务访问接口。

```go
type LumenClient struct {
    config    *config.Config          // 客户端配置
    discovery *MDNSDiscovery         // 服务发现组件
    pool      *GRPCConnectionPool     // 连接池
    balancer  LoadBalancer          // 负载均衡器
    logger    *zap.Logger           // 日志记录器
    metrics   *ClientMetrics        // 性能指标
}
```

### NodeInfo
节点信息结构，包含节点的完整描述。

```go
type NodeInfo struct {
    ID           string                 // 节点唯一标识
    Name         string                 // 节点名称
    Address      string                 // 网络地址
    Status       NodeStatus            // 节点状态
    Capabilities []*pb.Capability      // 服务能力
    Models       []*ModelInfo          // 支持的模型
    Load         *NodeLoad             // 当前负载
    Stats        *NodeStats            // 统计信息
    LastSeen     time.Time             // 最后活跃时间
}
```

### 接口定义

#### LoadBalancer
负载均衡器接口，支持多种选择策略。

```go
type LoadBalancer interface {
    SelectNode(ctx context.Context, task string) (*NodeInfo, error)
    UpdateNodes(nodes []*NodeInfo)
    GetStats() LoadBalancerStats
    Close() error
}
```

#### ServiceDiscovery
服务发现接口，支持不同的发现机制。

```go
type ServiceDiscovery interface {
    Start(ctx context.Context) error
    Stop()
    GetNodes() []*NodeInfo
    GetNode(id string) (*NodeInfo, bool)
    Watch(callback func([]*NodeInfo)) error
}
```

## 使用指南

### 基础用法

#### 1. 创建客户端

```go
import (
    "github.com/edwinzhancn/lumen-sdk/pkg/client"
    "github.com/edwinzhancn/lumen-sdk/pkg/config"
    "go.uber.org/zap"
)

// 使用默认配置
logger, _ := zap.NewDevelopment()
config := config.DefaultConfig()
client, err := client.NewLumenClient(config, logger)
if err != nil {
    log.Fatal("Failed to create client:", err)
}
```

#### 2. 启动客户端

```go
ctx := context.Background()
if err := client.Start(ctx); err != nil {
    log.Fatal("Failed to start client:", err)
}
defer client.Close()
```

#### 3. 执行推理请求

```go
import pb "github.com/edwinzhancn/lumen-sdk/proto"

// 创建推理请求
req := &pb.InferRequest{
    Task:      "text-generation",
    ModelId:   "gpt-3.5",
    Data:      []byte("Hello, world!"),
    MimeType:  "text/plain",
    Timeout:   30 * 1000, // 30秒超时
}

// 同步推理
resp, err := client.Infer(ctx, req)
if err != nil {
    log.Printf("Inference failed: %v", err)
} else {
    fmt.Printf("Result: %s\n", string(resp.Result))
}
```

#### 4. 流式推理

```go
// 流式推理
respChan, err := client.InferStream(ctx, req)
if err != nil {
    log.Fatal("Stream inference failed:", err)
}

for resp := range respChan {
    fmt.Printf("Chunk: %s\n", string(resp.Result))
    if resp.IsFinal {
        break
    }
}
```

### 高级配置

#### 1. 自定义负载均衡策略

```go
// 创建带指定负载均衡策略的客户端
client, err := client.NewLumenClientWithBalancer(
    config,
    logger,
    client.LeastLoaded, // 最少负载策略
)
```

#### 2. 自定义配置

```go
config := &config.Config{
    Discovery: config.DiscoveryConfig{
        ServiceType:  "_lumen._tcp",
        ScanInterval: 30 * time.Second,
        MaxNodes:     20,
        Timeout:      5 * time.Second,
    },
    Connection: config.ConnectionConfig{
        MaxConnections: 10,
        ConnectionTTL:  10 * time.Minute,
        DialTimeout:    5 * time.Second,
    },
    Retry: config.RetryConfig{
        MaxAttempts: 3,
        Backoff:     100 * time.Millisecond,
        MaxBackoff:  5 * time.Second,
    },
}
```

### 监控和管理

#### 1. 获取节点信息

```go
// 获取所有节点
nodes := client.GetNodes()
fmt.Printf("Total nodes: %d\n", len(nodes))

// 获取特定节点
if node, exists := client.GetNode("node-001"); exists {
    fmt.Printf("Node %s: %s (%s)\n", node.Name, node.Address, node.Status)
}

// 获取节点能力
capabilities, err := client.GetCapabilities(ctx, "node-001")
if err == nil {
    for _, cap := range capabilities {
        fmt.Printf("Service: %s, Models: %v\n", cap.ServiceName, cap.ModelIds)
    }
}
```

#### 2. 监控节点变化

```go
import "github.com/edwinzhancn/lumen-sdk/pkg/discovery"

// 监听节点变化
err := client.WatchNodes(func(nodes []*discovery.NodeInfo) {
    fmt.Printf("Nodes updated: %d active, %d total\n",
        countActiveNodes(nodes), len(nodes))

    for _, node := range nodes {
        if node.IsActive() {
            fmt.Printf("✓ %s (%s)\n", node.Name, node.Address)
        } else {
            fmt.Printf("✗ %s (%s) - %s\n", node.Name, node.Address, node.Status)
        }
    }
})
```

`GetNodes`, `GetNode`, and `WatchNodes` now expose discovery-layer node types from `pkg/discovery`.

#### 3. 性能指标

```go
// 获取客户端指标
metrics := client.GetMetrics()
fmt.Printf("Requests: %d total, %d success, %.2f%% success rate\n",
    metrics.TotalRequests,
    metrics.SuccessfulRequests,
    float64(metrics.SuccessfulRequests)/float64(metrics.TotalRequests)*100)
fmt.Printf("Average latency: %v, Throughput: %.2f qps\n",
    metrics.AverageLatency,
    metrics.ThroughputQPS)

// 获取负载均衡器统计
balancerStats := client.GetBalancerStats()
fmt.Printf("Balancer: %s, Nodes: %d/%d, Selections: %d\n",
    balancerStats.Strategy,
    balancerStats.ActiveNodes,
    balancerStats.TotalNodes,
    balancerStats.SelectionsCount)
```

## 负载均衡策略

### 1. RoundRobin (轮询)
按顺序轮流选择节点，适合节点性能相近的场景。

```go
client, _ := client.NewLumenClientWithBalancer(config, logger, client.RoundRobin)
```

### 2. Random (随机)
随机选择节点，简单高效。

```go
client, _ := client.NewLumenClientWithBalancer(config, logger, client.Random)
```

### 3. LeastLoaded (最少负载)
选择当前负载最小的节点，适合异构环境。

```go
client, _ := client.NewLumenClientWithBalancer(config, logger, client.LeastLoaded)
```

## 错误处理

### 常见错误类型

```go
// 连接错误
if err != nil {
    if strings.Contains(err.Error(), "connection refused") {
        // 节点不可达
        log.Println("Node unreachable, trying fallback...")
    } else if strings.Contains(err.Error(), "timeout") {
        // 请求超时
        log.Println("Request timeout, consider increasing timeout...")
    }
}

// 服务发现错误
if err != nil {
    if strings.Contains(err.Error(), "no nodes available") {
        // 没有可用节点
        log.Println("No nodes available, check service discovery...")
    }
}
```

### 重试机制

客户端内置了指数退避重试机制：

```go
config := &config.Config{
    Retry: config.RetryConfig{
        MaxAttempts: 3,              // 最大重试次数
        Backoff:     100 * time.Millisecond,  // 初始退避时间
        MaxBackoff:  5 * time.Second,  // 最大退避时间
        Multiplier:  2.0,             // 退避倍数
    },
}
```

## 最佳实践

### 1. 生命周期管理

```go
// 正确的生命周期管理
func main() {
    // 创建客户端
    client, err := client.NewLumenClient(config, logger)
    if err != nil {
        log.Fatal(err)
    }

    // 启动客户端
    ctx := context.Background()
    if err := client.Start(ctx); err != nil {
        log.Fatal(err)
    }
    defer client.Close() // 确保资源释放

    // 使用客户端...
}
```

### 2. 上下文管理

```go
// 使用带超时的上下文
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

resp, err := client.Infer(ctx, req)
if err != nil {
    if err == context.DeadlineExceeded {
        log.Println("Request timeout")
    }
}
```

### 3. 连接池优化

```go
// 根据并发需求调整连接池大小
config := &config.Config{
    Connection: config.ConnectionConfig{
        MaxConnections: 50,      // 最大连接数
        ConnectionTTL:  10 * time.Minute,  // 连接TTL
        IdleTimeout:    5 * time.Minute,  // 空闲超时
    },
}
```

## 性能调优

### 1. 连接池调优

- **MaxConnections**: 根据并发请求数设置，通常为并发数的1.5倍
- **ConnectionTTL**: 长连接TTL，建议5-15分钟
- **IdleTimeout**: 空闲连接超时，建议3-5分钟

### 2. 负载均衡调优

- **策略选择**: 同构环境用RoundRobin，异构环境用LeastLoaded
- **健康检查**: 适当调整检查间隔，平衡性能和准确性

### 3. 监控指标

关注以下关键指标：
- **成功率**: 应保持在95%以上
- **平均延迟**: 根据业务需求，通常应<100ms
- **节点利用率**: 保持60-80%，避免过载

## 故障排查

### 1. 服务发现问题

```bash
# 检查mDNS服务是否正常
dns-sd -B _lumen._tcp

# 检查网络连通性
ping <node-address>
```

### 2. 连接问题

```go
// 检查节点状态
nodes := client.GetNodes()
for _, node := range nodes {
    fmt.Printf("Node %s: %s, Last seen: %v\n",
        node.Name, node.Status, node.LastSeen)
}
```

### 3. 性能问题

```go
// 检查节点负载
for _, node := range client.GetNodes() {
    if node.Load != nil {
        fmt.Printf("Node %s: CPU=%.2f%%, Memory=%.2f%%\n",
            node.Name, node.Load.CPU*100, node.Load.Memory*100)
    }
}
```

## API参考

### 核心方法

| 方法 | 描述 | 参数 | 返回 |
|------|------|------|------|
| `Start(ctx)` | 启动客户端 | context.Context | error |
| `Stop()` | 停止客户端 | - | error |
| `Infer(ctx, req)` | 同步推理 | context.Context, *pb.InferRequest | *pb.InferResponse, error |
| `InferStream(ctx, req)` | 流式推理 | context.Context, *pb.InferRequest | <-chan *pb.InferResponse, error |
| `GetNodes()` | 获取节点列表 | - | []*NodeInfo |
| `GetNode(id)` | 获取特定节点 | string | *NodeInfo, bool |
| `GetCapabilities(ctx, nodeID)` | 获取节点能力 | context.Context, string | []*pb.Capability, error |
| `GetMetrics()` | 获取性能指标 | - | *ClientMetrics |

### 配置选项

详细配置选项请参考 `pkg/config` 模块的文档。

## 示例代码

完整的使用示例请参考 `examples/` 目录下的示例程序。

---

## 更新日志

### v1.0.0
- 初始版本发布
- 支持mDNS服务发现
- 实现多种负载均衡策略
- 提供完整的连接池管理
- 集成监控和指标收集

### v1.1.0 (计划中)
- 支持Consul服务发现
- 添加自定义负载均衡策略
- 增强错误处理和重试机制
- 支持熔断器模式

## 贡献指南

欢迎贡献代码！请阅读项目根目录的 `CONTRIBUTING.md` 文件了解贡献流程。

## 许可证

本项目采用 MIT 许可证，详情请参见 `LICENSE` 文件。
