package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the configuration for the Lumen SDK.
//
// Configuration can be loaded from a YAML file with environment variable
// overrides via LoadConfig, or created programmatically via DefaultConfig.
type Config struct {
	Discovery DiscoveryConfig `yaml:"discovery" json:"discovery"`
	Server    ServerConfig    `yaml:"server" json:"server"`
	Logging   LoggingConfig   `yaml:"logging" json:"logging"`
	Chunk     ChunkConfig     `yaml:"chunk" json:"chunk"`
}

// DiscoveryConfig controls service discovery for finding ML nodes.
type DiscoveryConfig struct {
	Enabled               bool          `yaml:"enabled" json:"enabled"`
	ServiceType           string        `yaml:"service_type" json:"service_type"`
	Domain                string        `yaml:"domain" json:"domain"`
	DeploymentID          string        `yaml:"deployment_id" json:"deployment_id"`
	ResolveTimeout        time.Duration `yaml:"resolve_timeout" json:"resolve_timeout"`
	ConnectTimeout        time.Duration `yaml:"connect_timeout" json:"connect_timeout"`
	RediscoveryBackoffMin time.Duration `yaml:"rediscovery_backoff_min" json:"rediscovery_backoff_min"`
	RediscoveryBackoffMax time.Duration `yaml:"rediscovery_backoff_max" json:"rediscovery_backoff_max"`
	ScanInterval          time.Duration `yaml:"scan_interval" json:"scan_interval"` // mDNS poll interval: how often to re-query for services.
	NodeTimeout           time.Duration `yaml:"node_timeout" json:"node_timeout"`   // Deprecated: DNS-SD is not operational liveness.
	MDNSEnabled           bool          `yaml:"mdns_enabled" json:"mdns_enabled"`
	HubURL                string        `yaml:"hub_url" json:"hub_url"`
}

// ServerConfig holds REST server configuration.
type ServerConfig struct {
	REST RESTConfig `yaml:"rest" json:"rest"`
}

// RESTConfig configures the REST API server.
type RESTConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Host    string `yaml:"host" json:"host"`
	Port    int    `yaml:"port" json:"port"`
	CORS    bool   `yaml:"cors" json:"cors"`
}

// LoggingConfig configures logging output.
type LoggingConfig struct {
	Level  string `yaml:"level" json:"level"`
	Format string `yaml:"format" json:"format"`
	Output string `yaml:"output" json:"output"`
}

// ChunkConfig controls automatic payload chunking.
type ChunkConfig struct {
	EnableAuto    bool `yaml:"enable_auto" json:"enable_auto"`
	Threshold     int  `yaml:"threshold" json:"threshold"`
	MaxChunkBytes int  `yaml:"max_chunk_bytes" json:"max_chunk_bytes"`
}

// LoadConfig loads configuration from a YAML file with environment overrides.
// If configPath is empty, DefaultConfig is used with env overrides.
func LoadConfig(configPath string) (*Config, error) {
	config := DefaultConfig()

	if configPath != "" {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("read config %s: %w", configPath, err)
		}
		if err := yaml.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("parse config %s: %w", configPath, err)
		}
	}

	if err := config.LoadFromEnv(); err != nil {
		return nil, fmt.Errorf("env overrides: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return config, nil
}

// LoadFromEnv applies LUMEN_* environment variable overrides.
func (c *Config) LoadFromEnv() error {
	if os.Getenv("LUMEN_DISCOVERY_ENABLED") != "" {
		c.Discovery.Enabled = os.Getenv("LUMEN_DISCOVERY_ENABLED") == "true"
	}
	if v := os.Getenv("LUMEN_DISCOVERY_SERVICE_TYPE"); v != "" {
		c.Discovery.ServiceType = v
	}
	if v := os.Getenv("LUMEN_DISCOVERY_DOMAIN"); v != "" {
		c.Discovery.Domain = v
	}
	if v := os.Getenv("LUMEN_DISCOVERY_DEPLOYMENT_ID"); v != "" {
		c.Discovery.DeploymentID = v
	}
	if v := os.Getenv("LUMEN_DISCOVERY_RESOLVE_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			c.Discovery.ResolveTimeout = d
		}
	}
	if v := os.Getenv("LUMEN_DISCOVERY_CONNECT_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			c.Discovery.ConnectTimeout = d
		}
	}
	if v := os.Getenv("LUMEN_DISCOVERY_REDISCOVERY_BACKOFF_MIN"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			c.Discovery.RediscoveryBackoffMin = d
		}
	}
	if v := os.Getenv("LUMEN_DISCOVERY_REDISCOVERY_BACKOFF_MAX"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			c.Discovery.RediscoveryBackoffMax = d
		}
	}
	if os.Getenv("LUMEN_DISCOVERY_MDNS_ENABLED") != "" {
		c.Discovery.MDNSEnabled = os.Getenv("LUMEN_DISCOVERY_MDNS_ENABLED") == "true"
	}
	if v := os.Getenv("LUMEN_DISCOVERY_HUB_URL"); v != "" {
		c.Discovery.HubURL = v
	}
	if v := os.Getenv("LUMEN_REST_HOST"); v != "" {
		c.Server.REST.Host = v
	}
	if v := os.Getenv("LUMEN_REST_PORT"); v != "" {
		if p, err := parsePort(v); err == nil {
			c.Server.REST.Port = p
		}
	}
	if v := os.Getenv("LUMEN_LOG_LEVEL"); v != "" {
		c.Logging.Level = v
	}
	if v := os.Getenv("LUMEN_LOG_FORMAT"); v != "" {
		c.Logging.Format = v
	}
	if v := os.Getenv("LUMEN_LOG_OUTPUT"); v != "" {
		c.Logging.Output = v
	}
	return nil
}

// Validate checks for configuration correctness.
func (c *Config) Validate() error {
	if c.Discovery.Enabled {
		if c.Discovery.ServiceType == "" {
			return fmt.Errorf("discovery.service_type is required when enabled")
		}
		if c.Discovery.DeploymentID == "" {
			return fmt.Errorf("discovery.deployment_id is required when enabled")
		}
		if c.Discovery.ResolveTimeout <= 0 {
			return fmt.Errorf("discovery.resolve_timeout must be positive")
		}
		if c.Discovery.ConnectTimeout <= 0 {
			return fmt.Errorf("discovery.connect_timeout must be positive")
		}
		if c.Discovery.RediscoveryBackoffMin <= 0 {
			return fmt.Errorf("discovery.rediscovery_backoff_min must be positive")
		}
		if c.Discovery.RediscoveryBackoffMax < c.Discovery.RediscoveryBackoffMin {
			return fmt.Errorf("discovery.rediscovery_backoff_max must be >= rediscovery_backoff_min")
		}
		if c.Discovery.ScanInterval < 0 {
			return fmt.Errorf("discovery.scan_interval must be non-negative")
		}
		if c.Discovery.NodeTimeout < 0 {
			return fmt.Errorf("discovery.node_timeout must be non-negative")
		}
	}
	if c.Server.REST.Enabled {
		if c.Server.REST.Port <= 0 || c.Server.REST.Port > 65535 {
			return fmt.Errorf("rest.port must be in 1-65535")
		}
	}
	if !validLogLevel[c.Logging.Level] {
		return fmt.Errorf("invalid log level: %s", c.Logging.Level)
	}
	if !validLogFormat[c.Logging.Format] {
		return fmt.Errorf("invalid log format: %s", c.Logging.Format)
	}
	return nil
}

var validLogLevel = map[string]bool{"debug": true, "info": true, "warn": true, "error": true, "fatal": true}
var validLogFormat = map[string]bool{"json": true, "text": true}

// SaveConfig writes the configuration to a YAML file.
func (c *Config) SaveConfig(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	return nil
}

func parsePort(s string) (int, error) {
	var port int
	if _, err := fmt.Sscanf(s, "%d", &port); err != nil {
		return 0, err
	}
	return port, nil
}
