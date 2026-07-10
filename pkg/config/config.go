package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the configuration for the Lumen SDK.
//
// Configuration can be loaded from a YAML file with environment variable
// overrides via LoadConfig, or created programmatically via DefaultConfig.
type Config struct {
	Discovery DiscoveryConfig `yaml:"discovery" json:"discovery"`
	Broker    BrokerConfig    `yaml:"broker" json:"broker"`
	Logging   LoggingConfig   `yaml:"logging" json:"logging"`
	Chunk     ChunkConfig     `yaml:"chunk" json:"chunk"`
}

// DiscoveryConfig controls service discovery for finding ML nodes.
//
// The three discovery backends (mDNS, Broker push via BrokerURL, StaticNodes)
// are additive: every configured backend runs and their node events are
// merged. At least one must be configured when discovery is enabled.
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
	MDNSEnabled           bool          `yaml:"mdns_enabled" json:"mdns_enabled"`
	// BrokerURL is the base URL of a Lumen Host Broker exposing the
	// /v1/nodes/watch push-discovery endpoint.
	BrokerURL string `yaml:"broker_url" json:"broker_url"`
	// StaticNodes pins node gRPC endpoints ("host:port") that are always
	// resolved without any dynamic discovery. Connection health is still
	// managed by the pool; entries only need to be reachable eventually.
	StaticNodes []string `yaml:"static_nodes" json:"static_nodes"`
}

// EffectiveBrokerURL returns the configured Broker push-discovery URL.
func (c DiscoveryConfig) EffectiveBrokerURL() string {
	return c.BrokerURL
}

// BrokerConfig configures the Host Broker control-plane server.
type BrokerConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Host    string `yaml:"host" json:"host"`
	Port    int    `yaml:"port" json:"port"`
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
		v, err := strconv.ParseBool(os.Getenv("LUMEN_DISCOVERY_ENABLED"))
		if err != nil {
			return fmt.Errorf("LUMEN_DISCOVERY_ENABLED: %w", err)
		}
		c.Discovery.Enabled = v
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
		d, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("LUMEN_DISCOVERY_RESOLVE_TIMEOUT: %w", err)
		}
		c.Discovery.ResolveTimeout = d
	}
	if v := os.Getenv("LUMEN_DISCOVERY_CONNECT_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("LUMEN_DISCOVERY_CONNECT_TIMEOUT: %w", err)
		}
		c.Discovery.ConnectTimeout = d
	}
	if v := os.Getenv("LUMEN_DISCOVERY_REDISCOVERY_BACKOFF_MIN"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("LUMEN_DISCOVERY_REDISCOVERY_BACKOFF_MIN: %w", err)
		}
		c.Discovery.RediscoveryBackoffMin = d
	}
	if v := os.Getenv("LUMEN_DISCOVERY_REDISCOVERY_BACKOFF_MAX"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("LUMEN_DISCOVERY_REDISCOVERY_BACKOFF_MAX: %w", err)
		}
		c.Discovery.RediscoveryBackoffMax = d
	}
	if os.Getenv("LUMEN_DISCOVERY_MDNS_ENABLED") != "" {
		v, err := strconv.ParseBool(os.Getenv("LUMEN_DISCOVERY_MDNS_ENABLED"))
		if err != nil {
			return fmt.Errorf("LUMEN_DISCOVERY_MDNS_ENABLED: %w", err)
		}
		c.Discovery.MDNSEnabled = v
	}
	if v := os.Getenv("LUMEN_DISCOVERY_BROKER_URL"); v != "" {
		c.Discovery.BrokerURL = v
	}
	if v := os.Getenv("LUMEN_DISCOVERY_STATIC_NODES"); v != "" {
		var nodes []string
		for _, part := range strings.Split(v, ",") {
			if part = strings.TrimSpace(part); part != "" {
				nodes = append(nodes, part)
			}
		}
		c.Discovery.StaticNodes = nodes
	}
	if v := os.Getenv("LUMEN_BROKER_HOST"); v != "" {
		c.Broker.Host = v
	}
	if v := os.Getenv("LUMEN_BROKER_PORT"); v != "" {
		p, err := parsePort(v)
		if err != nil {
			return fmt.Errorf("LUMEN_BROKER_PORT: %w", err)
		}
		c.Broker.Port = p
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
		for _, node := range c.Discovery.StaticNodes {
			if _, _, err := net.SplitHostPort(strings.TrimSpace(node)); err != nil {
				return fmt.Errorf("discovery.static_nodes entry %q must be host:port: %w", node, err)
			}
		}
	}
	if c.Broker.Enabled {
		if c.Broker.Port <= 0 || c.Broker.Port > 65535 {
			return fmt.Errorf("broker.port must be in 1-65535")
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
	return strconv.Atoi(strings.TrimSpace(s))
}
