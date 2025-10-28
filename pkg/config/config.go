package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config Lumen SDK统一配置结构
type Config struct {
	Discovery    DiscoveryConfig    `yaml:"discovery" json:"discovery"`
	Connection   ConnectionConfig   `yaml:"connection" json:"connection"`
	Server       ServerConfig       `yaml:"server" json:"server"`
	LoadBalancer LoadBalancerConfig `yaml:"load_balancer" json:"load_balancer"`
	Logging      LoggingConfig      `yaml:"logging" json:"logging"`
	Monitoring   MonitoringConfig   `yaml:"monitoring" json:"monitoring"`
}

// DiscoveryConfig 服务发现配置
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
	REST     RESTConfig     `yaml:"rest" json:"rest"`
	MCP      MCPConfig      `yaml:"mcp" json:"mcp"`
	LLMTools LLMToolsConfig `yaml:"llmtools" json:"llmtools"`
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

// LoadConfig 从文件加载配置
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

// LoadFromEnv 从环境变量加载配置
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

// Validate 验证配置有效性
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
