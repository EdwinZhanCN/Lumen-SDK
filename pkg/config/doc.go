// Package config provides configuration management for the Lumen SDK.
//
// The config package handles all configuration aspects including:
//
//   - Default configurations for quick starts
//   - Preset configurations optimized for different deployment scenarios
//   - YAML file loading with environment variable overrides
//   - Configuration validation
//   - Service discovery, connection, server, and logging settings
//
// # Quick Start
//
// Use default configuration for development:
//
//	cfg := config.DefaultConfig()
//	client, err := client.NewLumenClient(cfg, logger)
//
// # Preset Configurations
//
// Choose from optimized presets based on your deployment:
//
//	// For edge devices (Raspberry Pi, IoT)
//	cfg, _ := config.PresetConfig("minimal")
//
//	// For personal computers
//	cfg, _ := config.PresetConfig("basic")
//
//	// For high-performance servers
//	cfg, _ := config.PresetConfig("brave")
//
// Available presets:
//   - "minimal": Edge devices with limited resources
//   - "basic": Personal computers with standard resources
//   - "lightweight": Small servers with moderate resources
//   - "brave": High-performance servers for production
//
// # Loading from Files
//
// Load configuration from YAML files:
//
//	cfg, err := config.LoadConfig("/etc/lumen/config.yaml")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// Example YAML configuration:
//
//	discovery:
//	  enabled: true
//	  service_type: "_lumen._tcp"
//	  scan_interval: 30s
//	server:
//	  rest:
//	    enabled: true
//	    host: "0.0.0.0"
//	    port: 5866
//	load_balancer:
//	  strategy: "round_robin"
//	  cache_enabled: true
//	logging:
//	  level: "info"
//	  format: "json"
//
// # Environment Variables
//
// Override configuration with environment variables:
//
//	LUMEN_REST_PORT=9090
//	LUMEN_LOG_LEVEL=debug
//	LUMEN_LOAD_BALANCER_STRATEGY=weighted
//
// All LUMEN_* environment variables are automatically loaded.
//
// # Custom Configuration
//
// Create and customize configuration programmatically:
//
//	cfg := config.DefaultConfig()
//	cfg.Server.REST.Port = 9090
//	cfg.LoadBalancer.Strategy = "weighted"
//	cfg.Chunk.Threshold = 2 << 20 // 2MB
//
//	if err := cfg.Validate(); err != nil {
//	    log.Fatal(err)
//	}
//
// # Role in Project
//
// The config package is the configuration hub for the entire SDK, controlling
// behavior of all components. It enables deployment flexibility through presets,
// file-based configuration, and environment variable overrides.
package config
