# Config Module

## 1. Objective

**Purpose**: 提供Lumen SDK的统一配置管理，支持多源配置加载、验证和环境变量覆盖。

**Why needed**: 
- 统一管理各模块配置参数，避免配置分散
- 支持文件、环境变量等多配置源的灵活组合
- 提供默认配置和参数验证，确保系统启动可靠
- 便于生产环境和开发环境的配置切换

**Coding Guidelines**:
- 保持配置结构的向后兼容性
- 为所有配置字段提供合理的默认值
- 使用清晰的字段命名，避免歧义
- 提供详细的验证错误信息

## 2. Module Structure

```
pkg/config/
├── config.go        # 主配置结构定义和加载逻辑
├── defaults.go      # 默认配置常量
├── validator.go     # 配置验证逻辑
├── config_test.go   # 配置测试用例
└── README.md       # 本文档
```

**配置层次结构**:
```
Config
├── Discovery (服务发现)
├── Connection (连接管理)
├── Server (服务配置)
│   ├── REST
│   ├── GRPC
│   ├── MCP
│   └── LLMTools
├── Dispatcher (分发器)
├── Logging (日志)
└── Monitoring (监控)
```

## 3. Module Members

### Core Structure
- `Config` - 主配置结构，包含所有配置子模块

### Sub-Configurations
- `DiscoveryConfig` - 服务发现配置 (mDNS、扫描间隔等)
- `ConnectionConfig` - 连接配置 (超时、缓冲区大小等)
- `ServerConfig` - 服务配置 (REST、gRPC、MCP端口等)
- `DispatcherConfig` - 分发器配置 (负载均衡策略、缓存等)
- `LoggingConfig` - 日志配置 (级别、格式、输出)
- `MonitoringConfig` - 监控配置 (指标端口、健康检查)

### Validation
- `ConfigError` - 配置错误类型，提供字段级错误信息
- `Validate()` - 基础配置验证
- `ValidateWithErrors()` - 详细错误信息验证

### Utilities
- `LoadConfig()` - 从文件加载配置
- `LoadFromEnv()` - 从环境变量覆盖配置
- `SaveConfig()` - 保存配置到文件

## 4. Usage

### Basic Usage

```go
// 加载配置文件
config, err := config.LoadConfig("config.yaml")
if err != nil {
    log.Fatal(err)
}

// 使用默认配置
config := config.DefaultConfig()

// 验证配置
if err := config.Validate(); err != nil {
    log.Fatal(err)
}
```

### Configuration File

```yaml
# config.yaml
discovery:
  enabled: true
  service_type: "_lumen._tcp"
  domain: "local"
  scan_interval: 30s
  node_timeout: 5m
  max_nodes: 20

connection:
  dial_timeout: 5s
  keep_alive: 30s
  max_message_size: 4194304  # 4MB
  insecure: true
  compression: true

server:
  rest:
    enabled: true
    host: "0.0.0.0"
    port: 8080
    cors: true
    timeout: 30s
  grpc:
    enabled: true
    host: "0.0.0.0"
    port: 50051

dispatcher:
  strategy: "round_robin"
  cache_enabled: true
  cache_ttl: 5m
  default_timeout: 30s
  health_check: true
  check_interval: 30s

logging:
  level: "info"
  format: "json"
  output: "stdout"

monitoring:
  enabled: false
  metrics_port: 9090
  health_port: 8081
```

### Environment Variables

```bash
# 环境变量覆盖配置示例
export LUMEN_DISCOVERY_ENABLED=false
export LUMEN_REST_HOST=127.0.0.1
export LUMEN_REST_PORT=9090
export LUMEN_GRPC_PORT=50052
export LUMEN_LOG_LEVEL=debug
export LUMEN_LOG_FORMAT=text
export LUMEN_LOG_OUTPUT=file
export LUMEN_CONNECTION_INSECURE=false
```

### Custom Configuration

```go
// 创建自定义配置
config := &config.Config{
    Discovery: config.DiscoveryConfig{
        Enabled:      true,
        ServiceType:  "_myservice._tcp",
        ScanInterval: 60 * time.Second,
        MaxNodes:     50,
    },
    Server: config.ServerConfig{
        REST: config.RESTConfig{
            Enabled: true,
            Host:    "localhost",
            Port:    8080,
            CORS:    true,
        },
        GRPC: config.GRPCConfig{
            Enabled: true,
            Host:    "localhost",
            Port:    50051,
        },
    },
    Logging: config.LoggingConfig{
        Level:  "debug",
        Format: "json",
        Output: "stdout",
    },
}

// 验证配置
if err := config.Validate(); err != nil {
    // 处理验证错误
    log.Fatal(err)
}
```

### Validation

```go
// 基础验证
err := config.Validate()
if err != nil {
    log.Printf("配置验证失败: %v", err)
}

// 详细错误验证
errors := config.ValidateWithErrors()
if len(errors) > 0 {
    for _, err := range errors {
        if configErr, ok := err.(*config.ConfigError); ok {
            log.Printf("字段 '%s' 错误: %s", configErr.Field, configErr.Message)
        }
    }
}
```

### Runtime Updates

```go
// 保存运行时配置修改
config.Logging.Level = "debug"
config.Monitoring.Enabled = true

err := config.SaveConfig("updated_config.yaml")
if err != nil {
    log.Printf("保存配置失败: %v", err)
}
```

### Common Configurations

```go
// 开发环境配置
devConfig := config.DefaultConfig()
devConfig.Logging.Level = "debug"
devConfig.Monitoring.Enabled = true
devConfig.Server.REST.Port = 8081

// 生产环境配置
prodConfig := config.DefaultConfig()
prodConfig.Logging.Level = "warn"
prodConfig.Logging.Output = "file"
prodConfig.Connection.Insecure = false
prodConfig.Monitoring.Enabled = true
```