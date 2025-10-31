package config

import (
	"fmt"
	"time"
)

// PresetConfig returns a configuration for the specified preset
func PresetConfig(preset string) (*Config, error) {
	switch preset {
	case "minimal":
		return minimalPreset(), nil
	case "basic":
		return basicPreset(), nil
	case "lightweight":
		return lightweightPreset(), nil
	case "brave":
		return bravePreset(), nil
	default:
		return nil, &InvalidPresetError{Preset: preset}
	}
}

// InvalidPresetError represents an error when an invalid preset is requested
type InvalidPresetError struct {
	Preset string
}

func (e *InvalidPresetError) Error() string {
	return fmt.Sprintf("invalid preset '%s'. Valid presets: minimal, basic, lightweight, brave", e.Preset)
}

// minimalPreset returns configuration for edge devices with limited resources
func minimalPreset() *Config {
	return &Config{
		Discovery: DiscoveryConfig{
			Enabled:      true,
			ServiceType:  "_lumen._tcp",
			Domain:       "local",
			ScanInterval: 60 * time.Second, // Less frequent scans to save CPU
			NodeTimeout:  10 * time.Minute, // Longer timeout for unstable connections
			MaxNodes:     5,                // Limit discovered nodes to reduce memory usage
		},
		Connection: ConnectionConfig{
			DialTimeout:    10 * time.Second, // Longer timeout for unreliable networks
			KeepAlive:      60 * time.Second, // Longer keep-alive to reduce reconnections
			MaxMessageSize: 2 * 1024 * 1024,  // 2MB limit to save memory
			Insecure:       false,
			Compression:    true, // Save bandwidth
		},
		Server: ServerConfig{
			REST: RESTConfig{
				Enabled: true,
				Host:    "0.0.0.0",
				Port:    8080,
				CORS:    false,            // Disable CORS to save CPU
				Timeout: 60 * time.Second, // Longer timeout for slow edge processing
			},
			MCP: MCPConfig{
				Enabled: false, // Disable MCP to save resources
			},
			LLMTools: LLMToolsConfig{
				Enabled: false, // Disable LLM tools to save resources
			},
		},
		LoadBalancer: LoadBalancerConfig{
			Strategy:       "round_robin",    // Simple strategy, minimal CPU
			CacheEnabled:   true,             // Cache to reduce repeated requests
			CacheTTL:       10 * time.Minute, // Longer cache to save bandwidth
			DefaultTimeout: 60 * time.Second, // Longer default timeout for edge processing
			HealthCheck:    false,            // Disable health checks to save CPU
			CheckInterval:  0,
		},
		Logging: LoggingConfig{
			Level:  "warn", // Only log warnings and errors
			Format: "text", // Text format saves CPU compared to JSON
			Output: "stdout",
		},
		Monitoring: MonitoringConfig{
			Enabled:     false, // Disable monitoring to save CPU/memory
			MetricsPort: 9090,
			HealthPort:  8081,
		},
		Chunk: ChunkConfig{
			EnableAuto:    true,
			Threshold:     256 * 1024, // 256 KiB threshold for edge devices
			MaxChunkBytes: 64 * 1024,  // 64 KiB per chunk
		},
	}
}

// basicPreset returns configuration for personal computers with standard resources
func basicPreset() *Config {
	return &Config{
		Discovery: DiscoveryConfig{
			Enabled:      true,
			ServiceType:  "_lumen._tcp",
			Domain:       "local",
			ScanInterval: 30 * time.Second, // Standard scan frequency
			NodeTimeout:  5 * time.Minute,  // Standard timeout
			MaxNodes:     20,               // Good limit for personal computers
		},
		Connection: ConnectionConfig{
			DialTimeout:    5 * time.Second,  // Standard timeout
			KeepAlive:      30 * time.Second, // Standard keep-alive
			MaxMessageSize: 4 * 1024 * 1024,  // 4MB limit
			Insecure:       false,
			Compression:    true, // Save bandwidth
		},
		Server: ServerConfig{
			REST: RESTConfig{
				Enabled: true,
				Host:    "0.0.0.0",
				Port:    8080,
				CORS:    true,             // Enable CORS for web interfaces
				Timeout: 30 * time.Second, // Standard timeout
			},
			MCP: MCPConfig{
				Enabled: false, // MCP disabled by default
			},
			LLMTools: LLMToolsConfig{
				Enabled: false, // LLM tools disabled by default
			},
		},
		LoadBalancer: LoadBalancerConfig{
			Strategy:       "round_robin",    // Simple and reliable
			CacheEnabled:   true,             // Enable caching for performance
			CacheTTL:       5 * time.Minute,  // Standard cache duration
			DefaultTimeout: 30 * time.Second, // Standard timeout
			HealthCheck:    true,             // Enable health checks
			CheckInterval:  10 * time.Second, // Standard check frequency
		},
		Logging: LoggingConfig{
			Level:  "info", // Standard logging level
			Format: "json", // JSON format for structured logging
			Output: "stdout",
		},
		Monitoring: MonitoringConfig{
			Enabled:     true, // Enable monitoring for observability
			MetricsPort: 9091, // Standard metrics port
			HealthPort:  9092, // Standard health check port
		},
		Chunk: ChunkConfig{
			EnableAuto:    true,
			Threshold:     1 << 20,    // 1 MiB threshold
			MaxChunkBytes: 256 * 1024, // 256 KiB per chunk
		},
	}
}

// lightweightPreset returns configuration for small computers with moderate resources
func lightweightPreset() *Config {
	return &Config{
		Discovery: DiscoveryConfig{
			Enabled:      true,
			ServiceType:  "_lumen._tcp",
			Domain:       "local",
			ScanInterval: 45 * time.Second, // Moderate scan frequency
			NodeTimeout:  5 * time.Minute,  // Standard timeout
			MaxNodes:     10,               // Reasonable limit for small computers
		},
		Connection: ConnectionConfig{
			DialTimeout:    5 * time.Second,  // Standard timeout
			KeepAlive:      30 * time.Second, // Standard keep-alive
			MaxMessageSize: 4 * 1024 * 1024,  // 4MB limit
			Insecure:       false,
			Compression:    true, // Save bandwidth
		},
		Server: ServerConfig{
			REST: RESTConfig{
				Enabled: true,
				Host:    "0.0.0.0",
				Port:    8080,
				CORS:    true,             // Enable CORS for web interfaces
				Timeout: 30 * time.Second, // Standard timeout
			},
			MCP: MCPConfig{
				Enabled: false, // Disable MCP to save resources
			},
			LLMTools: LLMToolsConfig{
				Enabled: false, // Disable LLM tools to save resources
			},
		},
		LoadBalancer: LoadBalancerConfig{
			Strategy:       "round_robin",    // Simple and efficient
			CacheEnabled:   true,             // Enable caching for performance
			CacheTTL:       5 * time.Minute,  // Standard cache duration
			DefaultTimeout: 30 * time.Second, // Standard timeout
			HealthCheck:    true,             // Enable health checks
			CheckInterval:  30 * time.Second, // Moderate check frequency
		},
		Logging: LoggingConfig{
			Level:  "info", // Standard logging level
			Format: "json", // JSON format for structured logging
			Output: "stdout",
		},
		Monitoring: MonitoringConfig{
			Enabled:     true, // Enable monitoring for health checks
			MetricsPort: 9091, // Standard metrics port
			HealthPort:  9092, // Standard health check port
		},
		Chunk: ChunkConfig{
			EnableAuto:    true,
			Threshold:     512 * 1024, // 512 KiB threshold
			MaxChunkBytes: 128 * 1024, // 128 KiB per chunk
		},
	}
}

// bravePreset returns configuration for high-performance server environments
func bravePreset() *Config {
	return &Config{
		Discovery: DiscoveryConfig{
			Enabled:      true,
			ServiceType:  "_lumen._tcp",
			Domain:       "local",
			ScanInterval: 15 * time.Second, // Frequent scans for rapid node discovery
			NodeTimeout:  2 * time.Minute,  // Short timeout for quick failure detection
			MaxNodes:     50,               // High limit for enterprise deployments
		},
		Connection: ConnectionConfig{
			DialTimeout:    3 * time.Second,  // Fast connection establishment
			KeepAlive:      15 * time.Second, // Shorter keep-alive for connection recycling
			MaxMessageSize: 8 * 1024 * 1024,  // 8MB limit for large payloads
			Insecure:       false,
			Compression:    true, // Save bandwidth
		},
		Server: ServerConfig{
			REST: RESTConfig{
				Enabled: true,
				Host:    "0.0.0.0",
				Port:    8080,
				CORS:    true,             // Enable CORS for web interfaces
				Timeout: 15 * time.Second, // Fast timeout for responsive services
			},
			MCP: MCPConfig{
				Enabled: true, // Enable MCP for enterprise integration
			},
			LLMTools: LLMToolsConfig{
				Enabled: true, // Enable LLM tools for advanced features
			},
		},
		LoadBalancer: LoadBalancerConfig{
			Strategy:       "least_connections", // Optimize for load distribution
			CacheEnabled:   true,                // Enable caching for performance
			CacheTTL:       2 * time.Minute,     // Short cache for fresher results
			DefaultTimeout: 15 * time.Second,    // Fast default timeout
			HealthCheck:    true,                // Aggressive health monitoring
			CheckInterval:  5 * time.Second,     // Frequent health checks
		},
		Logging: LoggingConfig{
			Level:  "info", // Standard logging level
			Format: "json", // JSON format for structured logging
			Output: "stdout",
		},
		Monitoring: MonitoringConfig{
			Enabled:     true, // Enable comprehensive monitoring
			MetricsPort: 9091, // Standard metrics port
			HealthPort:  9092, // Standard health check port
		},
		Chunk: ChunkConfig{
			EnableAuto:    true,
			Threshold:     4 << 20, // 4 MiB threshold for high-performance servers
			MaxChunkBytes: 1 << 20, // 1 MiB per chunk
		},
	}
}

// GetValidPresets returns a list of all valid preset names
func GetValidPresets() []string {
	return []string{"minimal", "basic", "lightweight", "brave"}
}

// IsValidPreset checks if the given preset name is valid
func IsValidPreset(preset string) bool {
	validPresets := GetValidPresets()
	for _, valid := range validPresets {
		if preset == valid {
			return true
		}
	}
	return false
}
