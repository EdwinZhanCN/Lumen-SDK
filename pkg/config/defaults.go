package config

import "time"

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Discovery: DiscoveryConfig{
			Enabled:      true,
			ServiceType:  "_lumen._tcp",
			Domain:       "local",
			ScanInterval: 30 * time.Second,
			NodeTimeout:  5 * time.Minute,
			MaxNodes:     20,
		},
		Connection: ConnectionConfig{
			DialTimeout:    5 * time.Second,
			KeepAlive:      30 * time.Second,
			MaxMessageSize: 4 * 1024 * 1024, // 4MB
			Insecure:       true,
			Compression:    true,
		},
		Server: ServerConfig{
			REST: RESTConfig{
				Enabled: true,
				Host:    "0.0.0.0",
				Port:    8080,
				CORS:    true,
				Timeout: 30 * time.Second,
			},

			MCP: MCPConfig{
				Enabled: true,
				Host:    "0.0.0.0",
				Port:    6000,
			},
			LLMTools: LLMToolsConfig{
				Enabled: true,
			},
		},
		LoadBalancer: LoadBalancerConfig{
			Strategy:       "round_robin",
			CacheEnabled:   true,
			CacheTTL:       5 * time.Minute,
			DefaultTimeout: 30 * time.Second,
			HealthCheck:    true,
			CheckInterval:  30 * time.Second,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		},
		Monitoring: MonitoringConfig{
			Enabled:     false,
			MetricsPort: 9090,
			HealthPort:  8081,
		},
	}
}
