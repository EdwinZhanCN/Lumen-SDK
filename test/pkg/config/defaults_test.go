package config_test

import (
	"testing"
	"time"

	"github.com/edwinzhancn/lumen-sdk/pkg/config"
)

func TestDefaultConfigValues(t *testing.T) {
	cfg := config.DefaultConfig()

	// Discovery
	if !cfg.Discovery.Enabled {
		t.Error("Expected Discovery.Enabled to be true")
	}
	if cfg.Discovery.ServiceType != "_lumen._tcp" {
		t.Errorf("Expected ServiceType '_lumen._tcp', got %s", cfg.Discovery.ServiceType)
	}
	if cfg.Discovery.Domain != "local" {
		t.Errorf("Expected Domain 'local', got %s", cfg.Discovery.Domain)
	}
	if cfg.Discovery.ScanInterval != 30*time.Second {
		t.Errorf("Expected ScanInterval 30s, got %v", cfg.Discovery.ScanInterval)
	}
	if cfg.Discovery.NodeTimeout != 5*time.Minute {
		t.Errorf("Expected NodeTimeout 5m, got %v", cfg.Discovery.NodeTimeout)
	}
	if cfg.Discovery.MaxNodes != 20 {
		t.Errorf("Expected MaxNodes 20, got %d", cfg.Discovery.MaxNodes)
	}

	// Connection
	if cfg.Connection.DialTimeout != 5*time.Second {
		t.Errorf("Expected DialTimeout 5s, got %v", cfg.Connection.DialTimeout)
	}
	if cfg.Connection.KeepAlive != 30*time.Second {
		t.Errorf("Expected KeepAlive 30s, got %v", cfg.Connection.KeepAlive)
	}
	if cfg.Connection.MaxMessageSize != 4*1024*1024 {
		t.Errorf("Expected MaxMessageSize 4MB, got %d", cfg.Connection.MaxMessageSize)
	}
	if !cfg.Connection.Insecure {
		t.Error("Expected Connection.Insecure to be true")
	}
	if !cfg.Connection.Compression {
		t.Error("Expected Connection.Compression to be true")
	}

	// Server REST
	if !cfg.Server.REST.Enabled {
		t.Error("Expected REST.Enabled to be true")
	}
	if cfg.Server.REST.Host != "0.0.0.0" {
		t.Errorf("Expected REST.Host '0.0.0.0', got %s", cfg.Server.REST.Host)
	}
	if cfg.Server.REST.Port != 8080 {
		t.Errorf("Expected REST.Port 8080, got %d", cfg.Server.REST.Port)
	}
	if !cfg.Server.REST.CORS {
		t.Error("Expected REST.CORS to be true")
	}
	if cfg.Server.REST.Timeout != 30*time.Second {
		t.Errorf("Expected REST.Timeout 30s, got %v", cfg.Server.REST.Timeout)
	}

	// Server MCP
	if !cfg.Server.MCP.Enabled {
		t.Error("Expected MCP.Enabled to be true")
	}
	if cfg.Server.MCP.Host != "0.0.0.0" {
		t.Errorf("Expected MCP.Host '0.0.0.0', got %s", cfg.Server.MCP.Host)
	}
	if cfg.Server.MCP.Port != 6000 {
		t.Errorf("Expected MCP.Port 6000, got %d", cfg.Server.MCP.Port)
	}

	// Server LLMTools
	if !cfg.Server.LLMTools.Enabled {
		t.Error("Expected LLMTools.Enabled to be true")
	}

	// LoadBalancer
	if cfg.LoadBalancer.Strategy != "round_robin" {
		t.Errorf("Expected Strategy 'round_robin', got %s", cfg.LoadBalancer.Strategy)
	}
	if !cfg.LoadBalancer.CacheEnabled {
		t.Error("Expected CacheEnabled to be true")
	}
	if cfg.LoadBalancer.CacheTTL != 5*time.Minute {
		t.Errorf("Expected CacheTTL 5m, got %v", cfg.LoadBalancer.CacheTTL)
	}
	if cfg.LoadBalancer.DefaultTimeout != 30*time.Second {
		t.Errorf("Expected DefaultTimeout 30s, got %v", cfg.LoadBalancer.DefaultTimeout)
	}
	if !cfg.LoadBalancer.HealthCheck {
		t.Error("Expected HealthCheck to be true")
	}
	if cfg.LoadBalancer.CheckInterval != 30*time.Second {
		t.Errorf("Expected CheckInterval 30s, got %v", cfg.LoadBalancer.CheckInterval)
	}

	// Logging
	if cfg.Logging.Level != "info" {
		t.Errorf("Expected Level 'info', got %s", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "json" {
		t.Errorf("Expected Format 'json', got %s", cfg.Logging.Format)
	}
	if cfg.Logging.Output != "stdout" {
		t.Errorf("Expected Output 'stdout', got %s", cfg.Logging.Output)
	}

	// Monitoring
	if cfg.Monitoring.Enabled {
		t.Error("Expected Monitoring.Enabled to be false")
	}
	if cfg.Monitoring.MetricsPort != 9090 {
		t.Errorf("Expected MetricsPort 9090, got %d", cfg.Monitoring.MetricsPort)
	}
	if cfg.Monitoring.HealthPort != 8081 {
		t.Errorf("Expected HealthPort 8081, got %d", cfg.Monitoring.HealthPort)
	}

	// Chunk
	if !cfg.Chunk.EnableAuto {
		t.Error("Expected Chunk.EnableAuto to be true")
	}
	if cfg.Chunk.Threshold != 1<<20 {
		t.Errorf("Expected Chunk.Threshold 1MiB, got %d", cfg.Chunk.Threshold)
	}
	if cfg.Chunk.MaxChunkBytes != 256*1024 {
		t.Errorf("Expected Chunk.MaxChunkBytes 256KiB, got %d", cfg.Chunk.MaxChunkBytes)
	}
}

func TestDefaultConfigIsValid(t *testing.T) {
	cfg := config.DefaultConfig()
	if err := cfg.Validate(); err != nil {
		t.Errorf("Default config should be valid, got error: %v", err)
	}

	errs := cfg.ValidateWithErrors()
	if len(errs) > 0 {
		t.Errorf("Default config should have no validation errors, got %d errors", len(errs))
		for _, err := range errs {
			t.Logf("  - %v", err)
		}
	}
}

func TestDefaultConfigStructure(t *testing.T) {
	cfg := config.DefaultConfig()

	if cfg == nil {
		t.Fatal("DefaultConfig() returned nil")
	}

	// Ensure all structs are initialized
	if cfg.Discovery.ServiceType == "" {
		t.Error("Discovery should be initialized")
	}
	if cfg.Server.REST.Port == 0 {
		t.Error("REST server should be initialized")
	}
	if cfg.LoadBalancer.Strategy == "" {
		t.Error("LoadBalancer should be initialized")
	}
	if cfg.Logging.Level == "" {
		t.Error("Logging should be initialized")
	}
}
