package config

import "time"

// DefaultConfig returns the default configuration for the Lumen SDK.
//
// This configuration provides sensible defaults suitable for development and
// small-scale deployments. It enables:
//   - Service discovery via mDNS
//   - REST API on port 8080 with CORS enabled
//   - Round-robin load balancing with caching
//   - Health checking of nodes
//   - JSON logging to stdout at info level
//   - Automatic payload chunking for data >1MB
//
// For production use, consider using one of the preset configurations
// (basic, lightweight, brave) or loading from a configuration file.
//
// Returns:
//   - *Config: Default configuration ready to use
//
// Role in project: Provides zero-configuration setup for quick starts and
// development. Most users start with DefaultConfig() and customize as needed.
//
// Example:
//
//	// Use default configuration
//	cfg := config.DefaultConfig()
//	client, err := client.NewLumenClient(cfg, logger)
//
//	// Or customize defaults
//	cfg := config.DefaultConfig()
//	cfg.Server.REST.Port = 9090
//	cfg.LoadBalancer.Strategy = "weighted"
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
		Chunk: ChunkConfig{
			EnableAuto:    true,
			Threshold:     1 << 20,    // 1 MiB: payloads larger than this will be chunked
			MaxChunkBytes: 256 * 1024, // 256 KiB per chunk
		},
	}
}
