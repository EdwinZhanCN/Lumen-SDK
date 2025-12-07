package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the unified configuration structure for the Lumen SDK.
//
// This is the central configuration object that controls all aspects of the SDK's
// behavior including:
//   - Service discovery and node management
//   - Connection settings and timeouts
//   - Server configurations (REST, MCP, LLMTools)
//   - Load balancing strategies
//   - Logging and monitoring
//   - Payload chunking
//
// Configuration can be loaded from YAML files, environment variables, or created
// programmatically. The SDK provides several preset configurations for common use cases.
//
// Role in project: Central configuration hub that controls the behavior of all SDK
// components. Proper configuration is essential for optimal performance, reliability,
// and resource utilization.
//
// Example:
//
//	// Use default configuration
//	cfg := config.DefaultConfig()
//
//	// Load from YAML file
//	cfg, err := config.LoadConfig("config.yaml")
//
//	// Load preset configuration
//	cfg := config.GetPresetConfig("basic")
//
//	// Customize configuration
//	cfg := config.DefaultConfig()
//	cfg.Server.REST.Port = 9090
//	cfg.LoadBalancer.Strategy = "weighted"
type Config struct {
	Discovery    DiscoveryConfig    `yaml:"discovery" json:"discovery"`
	Connection   ConnectionConfig   `yaml:"connection" json:"connection"`
	Server       ServerConfig       `yaml:"server" json:"server"`
	LoadBalancer LoadBalancerConfig `yaml:"load_balancer" json:"load_balancer"`
	Logging      LoggingConfig      `yaml:"logging" json:"logging"`
	Monitoring   MonitoringConfig   `yaml:"monitoring" json:"monitoring"`
	Chunk        ChunkConfig        `yaml:"chunk" json:"chunk"`
}

// ChunkConfig controls automatic payload chunking for large data transfers.
//
// Chunking breaks large payloads (images, videos, large text) into smaller pieces
// for efficient transmission over gRPC. This prevents connection timeouts and
// enables progressive processing on the ML node side.
//
// Role in project: Enables reliable handling of large payloads without hitting
// gRPC message size limits or network timeouts.
type ChunkConfig struct {
	EnableAuto    bool `yaml:"enable_auto" json:"enable_auto"`         // Enable automatic chunking based on threshold
	Threshold     int  `yaml:"threshold" json:"threshold"`             // Size in bytes to trigger chunking (e.g., 1<<20 = 1MiB)
	MaxChunkBytes int  `yaml:"max_chunk_bytes" json:"max_chunk_bytes"` // Maximum size per chunk (e.g., 256KB/512KB)
}

// DiscoveryConfig controls service discovery for finding ML nodes.
//
// The SDK uses mDNS (multicast DNS) for zero-configuration discovery of ML nodes
// on the local network. Nodes advertise their services, and clients automatically
// discover them without manual configuration.
//
// Role in project: Enables automatic discovery and management of distributed ML nodes,
// eliminating the need for static node configuration and enabling dynamic scaling.
type DiscoveryConfig struct {
	Enabled      bool          `yaml:"enabled" json:"enabled"`
	ServiceType  string        `yaml:"service_type" json:"service_type"`
	Domain       string        `yaml:"domain" json:"domain"`
	ScanInterval time.Duration `yaml:"scan_interval" json:"scan_interval"`
	NodeTimeout  time.Duration `yaml:"node_timeout" json:"node_timeout"`
	MaxNodes     int           `yaml:"max_nodes" json:"max_nodes"`
}

// ConnectionConfig 连接配置
type ConnectionConfig struct {
	DialTimeout    time.Duration `yaml:"dial_timeout" json:"dial_timeout"`
	KeepAlive      time.Duration `yaml:"keep_alive" json:"keep_alive"`
	MaxMessageSize int           `yaml:"max_message_size" json:"max_message_size"`
	Insecure       bool          `yaml:"insecure" json:"insecure"`
	Compression    bool          `yaml:"compression" json:"compression"`
}

// ServerConfig 服务配置
type ServerConfig struct {
	REST RESTConfig `yaml:"rest" json:"rest"`
	MCP  MCPConfig  `yaml:"mcp" json:"mcp"`
}

// RESTConfig REST API配置
type RESTConfig struct {
	Enabled bool          `yaml:"enabled" json:"enabled"`
	Host    string        `yaml:"host" json:"host"`
	Port    int           `yaml:"port" json:"port"`
	CORS    bool          `yaml:"cors" json:"cors"`
	Timeout time.Duration `yaml:"timeout" json:"timeout"`
}

// MCPConfig MCP协议配置
type MCPConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Host    string `yaml:"host" json:"host"`
	Port    int    `yaml:"port" json:"port"`
}

// LLMToolsConfig LLM工具配置
type LLMToolsConfig struct {
	Enabled bool `yaml:"enabled" json:"enabled"`
}

// LoadBalancerConfig 负载均衡器配置
type LoadBalancerConfig struct {
	Strategy       string        `yaml:"strategy" json:"strategy"`
	CacheEnabled   bool          `yaml:"cache_enabled" json:"cache_enabled"`
	CacheTTL       time.Duration `yaml:"cache_ttl" json:"cache_ttl"`
	DefaultTimeout time.Duration `yaml:"default_timeout" json:"default_timeout"`
	HealthCheck    bool          `yaml:"health_check" json:"health_check"`
	CheckInterval  time.Duration `yaml:"check_interval" json:"check_interval"`
}

// LoggingConfig 日志配置
type LoggingConfig struct {
	Level  string `yaml:"level" json:"level"`
	Format string `yaml:"format" json:"format"`
	Output string `yaml:"output" json:"output"`
}

// MonitoringConfig 监控配置
type MonitoringConfig struct {
	Enabled     bool `yaml:"enabled" json:"enabled"`
	MetricsPort int  `yaml:"metrics_port" json:"metrics_port"`
	HealthPort  int  `yaml:"health_port" json:"health_port"`
}

// LoadConfig loads configuration from a YAML file with environment variable overrides.
//
// This function performs the following steps:
//  1. Starts with default configuration
//  2. Loads and merges settings from the YAML file (if path provided)
//  3. Applies environment variable overrides
//  4. Validates the final configuration
//
// If configPath is empty, only default config with env overrides is used.
//
// Parameters:
//   - configPath: Path to YAML configuration file (empty string for defaults only)
//
// Returns:
//   - *Config: Loaded and validated configuration
//   - error: Non-nil if file reading, parsing, or validation fails
//
// Role in project: Primary method for loading production configurations from files.
// Supports the standard deployment pattern of base config + environment overrides.
//
// Example:
//
//	// Load from file
//	cfg, err := config.LoadConfig("/etc/lumen/config.yaml")
//	if err != nil {
//	    log.Fatalf("Failed to load config: %v", err)
//	}
//
//	// Use defaults with environment overrides
//	cfg, err := config.LoadConfig("")
//
//	// Create client with loaded config
//	client, err := client.NewLumenClient(cfg, logger)
func LoadConfig(configPath string) (*Config, error) {
	config := DefaultConfig()

	// 如果指定了配置文件路径，则加载文件
	if configPath != "" {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
		}

		if err := yaml.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("failed to parse config file %s: %w", configPath, err)
		}
	}

	// 环境变量覆盖
	if err := config.LoadFromEnv(); err != nil {
		return nil, fmt.Errorf("failed to load environment variables: %w", err)
	}

	// 验证配置
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return config, nil
}

// LoadFromEnv loads configuration from environment variables.
//
// This method reads environment variables with the LUMEN_ prefix and overrides
// the corresponding configuration fields. Supported variables include:
//   - LUMEN_DISCOVERY_ENABLED: Enable/disable service discovery
//   - LUMEN_REST_HOST: REST API host
//   - LUMEN_REST_PORT: REST API port
//   - LUMEN_LOAD_BALANCER_STRATEGY: Load balancing strategy
//   - LUMEN_LOG_LEVEL: Logging level (debug, info, warn, error)
//
// Returns:
//   - error: Non-nil if environment variable parsing fails
//
// Role in project: Enables 12-factor app configuration pattern where environment
// variables override file-based settings. Essential for containerized deployments.
//
// Example:
//
//	// Set environment variables
//	os.Setenv("LUMEN_REST_PORT", "9090")
//	os.Setenv("LUMEN_LOG_LEVEL", "debug")
//
//	cfg := config.DefaultConfig()
//	if err := cfg.LoadFromEnv(); err != nil {
//	    log.Fatal(err)
//	}
func (c *Config) LoadFromEnv() error {
	// 服务发现配置
	if os.Getenv("LUMEN_DISCOVERY_ENABLED") != "" {
		c.Discovery.Enabled = os.Getenv("LUMEN_DISCOVERY_ENABLED") == "true"
	}
	if serviceType := os.Getenv("LUMEN_DISCOVERY_SERVICE_TYPE"); serviceType != "" {
		c.Discovery.ServiceType = serviceType
	}
	if domain := os.Getenv("LUMEN_DISCOVERY_DOMAIN"); domain != "" {
		c.Discovery.Domain = domain
	}

	// 连接配置
	if os.Getenv("LUMEN_CONNECTION_INSECURE") != "" {
		c.Connection.Insecure = os.Getenv("LUMEN_CONNECTION_INSECURE") == "true"
	}

	// 服务配置
	if host := os.Getenv("LUMEN_REST_HOST"); host != "" {
		c.Server.REST.Host = host
	}
	if port := os.Getenv("LUMEN_REST_PORT"); port != "" {
		if p, err := parsePort(port); err == nil {
			c.Server.REST.Port = p
		}
	}

	// GRPC server removed - no longer needed

	// 负载均衡器配置
	if strategy := os.Getenv("LUMEN_LOAD_BALANCER_STRATEGY"); strategy != "" {
		c.LoadBalancer.Strategy = strategy
	}
	if os.Getenv("LUMEN_LOAD_BALANCER_CACHE_ENABLED") != "" {
		c.LoadBalancer.CacheEnabled = os.Getenv("LUMEN_LOAD_BALANCER_CACHE_ENABLED") == "true"
	}
	if ttl := os.Getenv("LUMEN_LOAD_BALANCER_CACHE_TTL"); ttl != "" {
		if d, err := time.ParseDuration(ttl); err == nil {
			c.LoadBalancer.CacheTTL = d
		}
	}
	if timeout := os.Getenv("LUMEN_LOAD_BALANCER_DEFAULT_TIMEOUT"); timeout != "" {
		if d, err := time.ParseDuration(timeout); err == nil {
			c.LoadBalancer.DefaultTimeout = d
		}
	}
	if os.Getenv("LUMEN_LOAD_BALANCER_HEALTH_CHECK") != "" {
		c.LoadBalancer.HealthCheck = os.Getenv("LUMEN_LOAD_BALANCER_HEALTH_CHECK") == "true"
	}
	if interval := os.Getenv("LUMEN_LOAD_BALANCER_CHECK_INTERVAL"); interval != "" {
		if d, err := time.ParseDuration(interval); err == nil {
			c.LoadBalancer.CheckInterval = d
		}
	}

	// 日志配置
	if level := os.Getenv("LUMEN_LOG_LEVEL"); level != "" {
		c.Logging.Level = level
	}
	if format := os.Getenv("LUMEN_LOG_FORMAT"); format != "" {
		c.Logging.Format = format
	}
	if output := os.Getenv("LUMEN_LOG_OUTPUT"); output != "" {
		c.Logging.Output = output
	}

	return nil
}

// Validate checks the configuration for correctness and consistency.
//
// This method validates all configuration fields including:
//   - Service discovery settings (intervals, timeouts, max nodes)
//   - Connection parameters (timeouts, message sizes)
//   - Server ports and timeouts
//   - Load balancer settings
//   - Logging configuration
//   - Monitoring ports
//
// Returns:
//   - error: Non-nil with descriptive message if any validation fails
//
// Role in project: Prevents runtime errors by catching configuration mistakes early.
// Always called during config loading to ensure the SDK operates with valid settings.
//
// Example:
//
//	cfg := &config.Config{...}
//	if err := cfg.Validate(); err != nil {
//	    log.Fatalf("Invalid configuration: %v", err)
//	}
func (c *Config) Validate() error {
	// 验证服务发现配置
	if c.Discovery.Enabled {
		if c.Discovery.ServiceType == "" {
			return fmt.Errorf("discovery service_type cannot be empty when discovery is enabled")
		}
		if c.Discovery.ScanInterval <= 0 {
			return fmt.Errorf("discovery scan_interval must be positive")
		}
		if c.Discovery.NodeTimeout <= 0 {
			return fmt.Errorf("discovery node_timeout must be positive")
		}
		if c.Discovery.MaxNodes <= 0 {
			return fmt.Errorf("discovery max_nodes must be positive")
		}
	}

	// 验证连接配置
	if c.Connection.DialTimeout <= 0 {
		return fmt.Errorf("connection dial_timeout must be positive")
	}
	if c.Connection.KeepAlive <= 0 {
		return fmt.Errorf("connection keep_alive must be positive")
	}
	if c.Connection.MaxMessageSize <= 0 {
		return fmt.Errorf("connection max_message_size must be positive")
	}

	// 验证服务配置
	if c.Server.REST.Enabled {
		if c.Server.REST.Port <= 0 || c.Server.REST.Port > 65535 {
			return fmt.Errorf("server rest port must be in range 1-65535")
		}
		if c.Server.REST.Timeout <= 0 {
			return fmt.Errorf("server rest timeout must be positive")
		}
	}

	if c.Server.MCP.Enabled {
		if c.Server.MCP.Port <= 0 || c.Server.MCP.Port > 65535 {
			return fmt.Errorf("server mcp port must be in range 1-65535")
		}
	}

	// 验证负载均衡器配置
	if c.LoadBalancer.Strategy == "" {
		return fmt.Errorf("load_balancer strategy cannot be empty")
	}
	if c.LoadBalancer.CacheEnabled && c.LoadBalancer.CacheTTL <= 0 {
		return fmt.Errorf("load_balancer cache_ttl must be positive when cache is enabled")
	}
	if c.LoadBalancer.DefaultTimeout <= 0 {
		return fmt.Errorf("load_balancer default_timeout must be positive")
	}
	if c.LoadBalancer.HealthCheck && c.LoadBalancer.CheckInterval <= 0 {
		return fmt.Errorf("load_balancer check_interval must be positive when health check is enabled")
	}

	// 验证日志配置
	validLogLevels := map[string]bool{
		"debug": true, "info": true, "warn": true, "error": true, "fatal": true,
	}
	if !validLogLevels[c.Logging.Level] {
		return fmt.Errorf("invalid log level: %s", c.Logging.Level)
	}

	validLogFormats := map[string]bool{
		"json": true, "text": true,
	}
	if !validLogFormats[c.Logging.Format] {
		return fmt.Errorf("invalid log format: %s", c.Logging.Format)
	}

	// 验证监控配置
	if c.Monitoring.Enabled {
		if c.Monitoring.MetricsPort <= 0 || c.Monitoring.MetricsPort > 65535 {
			return fmt.Errorf("monitoring metrics_port must be in range 1-65535")
		}
		if c.Monitoring.HealthPort <= 0 || c.Monitoring.HealthPort > 65535 {
			return fmt.Errorf("monitoring health_port must be in range 1-65535")
		}
	}

	return nil
}

// SaveConfig 保存配置到文件
func (c *Config) SaveConfig(configPath string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file %s: %w", configPath, err)
	}

	return nil
}

// String 返回配置的字符串表示（用于调试）
func (c *Config) String() string {
	data, _ := yaml.Marshal(c)
	return string(data)
}

// 辅助函数
func parsePort(portStr string) (int, error) {
	var port int
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		return 0, err
	}
	return port, nil
}
