package config

import (
	"fmt"
	"strings"
)

// ConfigError 配置错误类型
type ConfigError struct {
	Field   string
	Message string
}

func (e *ConfigError) Error() string {
	return fmt.Sprintf("config error in field '%s': %s", e.Field, e.Message)
}

// NewConfigError 创建配置错误
func NewConfigError(field, message string) *ConfigError {
	return &ConfigError{
		Field:   field,
		Message: message,
	}
}

// ValidateWithErrors 带详细错误信息的配置验证
func (c *Config) ValidateWithErrors() []error {
	var errors []error

	// 验证服务发现配置
	if c.Discovery.Enabled {
		if strings.TrimSpace(c.Discovery.ServiceType) == "" {
			errors = append(errors, NewConfigError("discovery.service_type", "cannot be empty when discovery is enabled"))
		}
		if c.Discovery.ScanInterval <= 0 {
			errors = append(errors, NewConfigError("discovery.scan_interval", "must be positive"))
		}
		if c.Discovery.NodeTimeout <= 0 {
			errors = append(errors, NewConfigError("discovery.node_timeout", "must be positive"))
		}
		if c.Discovery.MaxNodes <= 0 {
			errors = append(errors, NewConfigError("discovery.max_nodes", "must be positive"))
		}
	}

	// 验证连接配置
	if c.Connection.DialTimeout <= 0 {
		errors = append(errors, NewConfigError("connection.dial_timeout", "must be positive"))
	}
	if c.Connection.KeepAlive <= 0 {
		errors = append(errors, NewConfigError("connection.keep_alive", "must be positive"))
	}
	if c.Connection.MaxMessageSize <= 0 {
		errors = append(errors, NewConfigError("connection.max_message_size", "must be positive"))
	}

	// 验证服务配置
	if c.Server.REST.Enabled {
		if c.Server.REST.Port <= 0 || c.Server.REST.Port > 65535 {
			errors = append(errors, NewConfigError("server.rest.port", "must be in range 1-65535"))
		}
		if c.Server.REST.Timeout <= 0 {
			errors = append(errors, NewConfigError("server.rest.timeout", "must be positive"))
		}
		if strings.TrimSpace(c.Server.REST.Host) == "" {
			errors = append(errors, NewConfigError("server.rest.host", "cannot be empty"))
		}
	}

	// GRPC server removed - no validation needed

	if c.Server.MCP.Enabled {
		if c.Server.MCP.Port <= 0 || c.Server.MCP.Port > 65535 {
			errors = append(errors, NewConfigError("server.mcp.port", "must be in range 1-65535"))
		}
		if strings.TrimSpace(c.Server.MCP.Host) == "" {
			errors = append(errors, NewConfigError("server.mcp.host", "cannot be empty"))
		}
	}

	// 验证分发器配置
	validStrategies := map[string]bool{
		"round_robin": true, "random": true, "least_loaded": true, "weighted": true,
	}
	if !validStrategies[c.LoadBalancer.Strategy] {
		errors = append(errors, NewConfigError("load_balancer.strategy",
			fmt.Sprintf("must be one of: %s", strings.Join(getValidStrategies(), ", "))))
	}
	if c.LoadBalancer.CacheEnabled && c.LoadBalancer.CacheTTL <= 0 {
		errors = append(errors, NewConfigError("load_balancer.cache_ttl", "must be positive when cache is enabled"))
	}
	if c.LoadBalancer.DefaultTimeout <= 0 {
		errors = append(errors, NewConfigError("load_balancer.default_timeout", "must be positive"))
	}
	if c.LoadBalancer.HealthCheck && c.LoadBalancer.CheckInterval <= 0 {
		errors = append(errors, NewConfigError("load_balancer.check_interval", "must be positive when health check is enabled"))
	}

	// 验证日志配置
	validLogLevels := map[string]bool{
		"debug": true, "info": true, "warn": true, "error": true, "fatal": true,
	}
	if !validLogLevels[c.Logging.Level] {
		errors = append(errors, NewConfigError("logging.level",
			fmt.Sprintf("must be one of: %s", strings.Join(getValidLogLevels(), ", "))))
	}

	validLogFormats := map[string]bool{
		"json": true, "text": true,
	}
	if !validLogFormats[c.Logging.Format] {
		errors = append(errors, NewConfigError("logging.format",
			fmt.Sprintf("must be one of: %s", strings.Join(getValidLogFormats(), ", "))))
	}

	validLogOutputs := map[string]bool{
		"stdout": true, "stderr": true, "file": true,
	}
	if !validLogOutputs[c.Logging.Output] {
		errors = append(errors, NewConfigError("logging.output",
			fmt.Sprintf("must be one of: %s", strings.Join(getValidLogOutputs(), ", "))))
	}

	// 验证监控配置
	if c.Monitoring.Enabled {
		if c.Monitoring.MetricsPort <= 0 || c.Monitoring.MetricsPort > 65535 {
			errors = append(errors, NewConfigError("monitoring.metrics_port", "must be in range 1-65535"))
		}
		if c.Monitoring.HealthPort <= 0 || c.Monitoring.HealthPort > 65535 {
			errors = append(errors, NewConfigError("monitoring.health_port", "must be in range 1-65535"))
		}
	}

	// 验证 Chunk 配置
	// - 当启用自动 chunk 时，Threshold 应为非负数，MaxChunkBytes 应为正数并且不应过大
	if c.Chunk.EnableAuto {
		if c.Chunk.Threshold < 0 {
			errors = append(errors, NewConfigError("chunk.threshold", "must be non-negative"))
		}
		if c.Chunk.MaxChunkBytes <= 0 {
			errors = append(errors, NewConfigError("chunk.max_chunk_bytes", "must be positive when chunking is enabled"))
		}
		// 防止用户设置一个不合理的单 chunk 大小（例如超过 100MB），以避免内存问题
		if c.Chunk.MaxChunkBytes > 100<<20 {
			errors = append(errors, NewConfigError("chunk.max_chunk_bytes", "unreasonably large; must be <= 100MiB"))
		}
		// 建议：MaxChunkBytes 不应大于 Threshold（否则 chunk 无意义），我们将其作为警告级别的错误
		if c.Chunk.Threshold > 0 && c.Chunk.MaxChunkBytes > c.Chunk.Threshold {
			errors = append(errors, NewConfigError("chunk.max_chunk_bytes", "should not be greater than chunk.threshold"))
		}
	} else {
		// 如果未启用自动 chunk，仍然校验提供的数值是否合理（以便用户后续开启时不会出错）
		if c.Chunk.Threshold < 0 {
			errors = append(errors, NewConfigError("chunk.threshold", "must be non-negative"))
		}
		if c.Chunk.MaxChunkBytes < 0 {
			errors = append(errors, NewConfigError("chunk.max_chunk_bytes", "must be non-negative"))
		}
	}

	return errors
}

// 辅助函数
func getValidStrategies() []string {
	return []string{"round_robin", "random", "least_loaded", "weighted"}
}

func getValidLogLevels() []string {
	return []string{"debug", "info", "warn", "error", "fatal"}
}

func getValidLogFormats() []string {
	return []string{"json", "text"}
}

func getValidLogOutputs() []string {
	return []string{"stdout", "stderr", "file"}
}
