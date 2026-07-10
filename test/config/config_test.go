package config

import (
	"os"
	"testing"
	"time"

	config2 "github.com/edwinzhancn/lumen-sdk/pkg/config"
)

// Existing basic tests retained and extended with chunk-related tests.

func TestDefaultConfig(t *testing.T) {
	config := config2.DefaultConfig()

	if !config.Discovery.Enabled {
		t.Error("Default discovery should be enabled")
	}

	if config.Discovery.ServiceType != "_lumen._tcp" {
		t.Errorf("Expected service_type '_lumen._tcp', got '%s'", config.Discovery.ServiceType)
	}

	if config.Discovery.DeploymentID != "local" {
		t.Errorf("Expected deployment_id 'local', got '%s'", config.Discovery.DeploymentID)
	}

	if config.Discovery.ResolveTimeout != 10*time.Second {
		t.Errorf("Expected resolve_timeout 10s, got %s", config.Discovery.ResolveTimeout)
	}

	if config.Discovery.ConnectTimeout != 10*time.Second {
		t.Errorf("Expected connect_timeout 10s, got %s", config.Discovery.ConnectTimeout)
	}

	if config.Discovery.RediscoveryBackoffMin != 10*time.Second {
		t.Errorf("Expected rediscovery_backoff_min 10s, got %s", config.Discovery.RediscoveryBackoffMin)
	}

	if config.Discovery.RediscoveryBackoffMax != 2*time.Minute {
		t.Errorf("Expected rediscovery_backoff_max 2m, got %s", config.Discovery.RediscoveryBackoffMax)
	}

	if config.Server.REST.Port != 5866 {
		t.Errorf("Expected REST port 5866, got %d", config.Server.REST.Port)
	}

	if config.Logging.Level != "info" {
		t.Errorf("Expected log level 'info', got '%s'", config.Logging.Level)
	}
}

// New test for chunk config defaults

func TestChunkDefaults(t *testing.T) {
	cfg := config2.DefaultConfig()
	if !cfg.Chunk.EnableAuto {
		t.Error("Default chunk EnableAuto should be true")
	}
	if cfg.Chunk.Threshold != 1<<20 {
		t.Fatalf("Expected default chunk Threshold %d, got %d", 1<<20, cfg.Chunk.Threshold)
	}
	if cfg.Chunk.MaxChunkBytes != 256*1024 {
		t.Fatalf("Expected default MaxChunkBytes %d, got %d", 256*1024, cfg.Chunk.MaxChunkBytes)
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *config2.Config
		wantErr bool
	}{
		{
			name:   "valid config",
			config: config2.DefaultConfig(),
		},
		{
			name: "invalid discovery - empty service type",
			config: &config2.Config{
				Discovery: config2.DiscoveryConfig{
					Enabled:     true,
					ServiceType: "",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid discovery - negative scan interval",
			config: &config2.Config{
				Discovery: config2.DiscoveryConfig{
					Enabled:      true,
					ServiceType:  "_lumen._tcp",
					ScanInterval: -1 * time.Second,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid rest port",
			config: &config2.Config{
				Server: config2.ServerConfig{
					REST: config2.RESTConfig{
						Enabled: true,
						Port:    0,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid log level",
			config: &config2.Config{
				Logging: config2.LoggingConfig{
					Level: "invalid",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadFromEnv(t *testing.T) {
	// 设置环境变量
	os.Setenv("LUMEN_DISCOVERY_ENABLED", "false")
	os.Setenv("LUMEN_DISCOVERY_DEPLOYMENT_ID", "lab")
	os.Setenv("LUMEN_DISCOVERY_RESOLVE_TIMEOUT", "3s")
	os.Setenv("LUMEN_DISCOVERY_CONNECT_TIMEOUT", "4s")
	os.Setenv("LUMEN_DISCOVERY_REDISCOVERY_BACKOFF_MIN", "5s")
	os.Setenv("LUMEN_DISCOVERY_REDISCOVERY_BACKOFF_MAX", "30s")
	os.Setenv("LUMEN_REST_HOST", "127.0.0.1")
	os.Setenv("LUMEN_REST_PORT", "9090")
	os.Setenv("LUMEN_LOG_LEVEL", "debug")
	os.Setenv("LUMEN_LOG_FORMAT", "text")

	defer func() {
		// 清理环境变量
		os.Unsetenv("LUMEN_DISCOVERY_ENABLED")
		os.Unsetenv("LUMEN_DISCOVERY_DEPLOYMENT_ID")
		os.Unsetenv("LUMEN_DISCOVERY_RESOLVE_TIMEOUT")
		os.Unsetenv("LUMEN_DISCOVERY_CONNECT_TIMEOUT")
		os.Unsetenv("LUMEN_DISCOVERY_REDISCOVERY_BACKOFF_MIN")
		os.Unsetenv("LUMEN_DISCOVERY_REDISCOVERY_BACKOFF_MAX")
		os.Unsetenv("LUMEN_REST_HOST")
		os.Unsetenv("LUMEN_REST_PORT")
		os.Unsetenv("LUMEN_LOG_LEVEL")
		os.Unsetenv("LUMEN_LOG_FORMAT")
	}()

	config := config2.DefaultConfig()
	err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}

	if config.Discovery.Enabled {
		t.Error("Expected discovery to be disabled from env")
	}

	if config.Discovery.DeploymentID != "lab" {
		t.Errorf("Expected deployment_id 'lab', got '%s'", config.Discovery.DeploymentID)
	}

	if config.Discovery.ResolveTimeout != 3*time.Second {
		t.Errorf("Expected resolve_timeout 3s, got %s", config.Discovery.ResolveTimeout)
	}

	if config.Discovery.ConnectTimeout != 4*time.Second {
		t.Errorf("Expected connect_timeout 4s, got %s", config.Discovery.ConnectTimeout)
	}

	if config.Discovery.RediscoveryBackoffMin != 5*time.Second {
		t.Errorf("Expected rediscovery_backoff_min 5s, got %s", config.Discovery.RediscoveryBackoffMin)
	}

	if config.Discovery.RediscoveryBackoffMax != 30*time.Second {
		t.Errorf("Expected rediscovery_backoff_max 30s, got %s", config.Discovery.RediscoveryBackoffMax)
	}

	if config.Server.REST.Host != "127.0.0.1" {
		t.Errorf("Expected REST host '127.0.0.1', got '%s'", config.Server.REST.Host)
	}

	if config.Server.REST.Port != 9090 {
		t.Errorf("Expected REST port 9090, got %d", config.Server.REST.Port)
	}

	if config.Logging.Level != "debug" {
		t.Errorf("Expected log level 'debug', got '%s'", config.Logging.Level)
	}

	if config.Logging.Format != "text" {
		t.Errorf("Expected log format 'text', got '%s'", config.Logging.Format)
	}
}

func TestEffectiveBrokerURL(t *testing.T) {
	tests := []struct {
		name      string
		brokerURL string
		hubURL    string
		want      string
	}{
		{"broker only", "http://broker:5866", "", "http://broker:5866"},
		{"hub only (deprecated)", "", "http://hub:5866", "http://hub:5866"},
		{"broker preferred over hub", "http://broker:5866", "http://hub:5866", "http://broker:5866"},
		{"neither set", "", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dc := config2.DiscoveryConfig{BrokerURL: tt.brokerURL, HubURL: tt.hubURL}
			if got := dc.EffectiveBrokerURL(); got != tt.want {
				t.Errorf("EffectiveBrokerURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidateRejectsConflictingBrokerAndHubURL(t *testing.T) {
	cfg := config2.DefaultConfig()
	cfg.Discovery.Enabled = false // isolate the conflict check from unrelated discovery validation
	cfg.Discovery.BrokerURL = "http://broker:5866"
	cfg.Discovery.HubURL = "http://hub:5866"

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected Validate() to reject differing broker_url and hub_url, got nil")
	}
}

func TestValidateAllowsEqualBrokerAndHubURL(t *testing.T) {
	cfg := config2.DefaultConfig()
	cfg.Discovery.Enabled = false
	cfg.Discovery.BrokerURL = "http://broker:5866"
	cfg.Discovery.HubURL = "http://broker:5866"

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() with equal broker_url/hub_url = %v, want nil", err)
	}
}

func TestLoadFromEnvBrokerURL(t *testing.T) {
	os.Setenv("LUMEN_DISCOVERY_BROKER_URL", "http://broker-from-env:5866")
	defer os.Unsetenv("LUMEN_DISCOVERY_BROKER_URL")

	config := config2.DefaultConfig()
	if err := config.LoadFromEnv(); err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}
	if config.Discovery.BrokerURL != "http://broker-from-env:5866" {
		t.Errorf("Expected broker_url from env, got %q", config.Discovery.BrokerURL)
	}
}

// TestLoadFromEnvHubURLStillWorks locks down that the deprecated
// LUMEN_DISCOVERY_HUB_URL environment variable keeps working unchanged.
func TestLoadFromEnvHubURLStillWorks(t *testing.T) {
	os.Setenv("LUMEN_DISCOVERY_HUB_URL", "http://hub-from-env:5866")
	defer os.Unsetenv("LUMEN_DISCOVERY_HUB_URL")

	config := config2.DefaultConfig()
	if err := config.LoadFromEnv(); err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}
	if config.Discovery.HubURL != "http://hub-from-env:5866" {
		t.Errorf("Expected hub_url from env, got %q", config.Discovery.HubURL)
	}
	if got := config.Discovery.EffectiveBrokerURL(); got != "http://hub-from-env:5866" {
		t.Errorf("EffectiveBrokerURL() = %q, want the deprecated hub_url value", got)
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	// 创建临时配置文件
	tmpFile := "/tmp/test_lumen_config.yaml"
	defer os.Remove(tmpFile)

	// 创建测试配置
	originalConfig := config2.DefaultConfig()
	originalConfig.Logging.Level = "debug"
	originalConfig.Server.REST.Port = 9090

	// 保存配置
	err := originalConfig.SaveConfig(tmpFile)
	if err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}

	// 加载配置
	loadedConfig, err := config2.LoadConfig(tmpFile)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	// 验证配置
	if loadedConfig.Logging.Level != "debug" {
		t.Errorf("Expected log level 'debug', got '%s'", loadedConfig.Logging.Level)
	}

	if loadedConfig.Server.REST.Port != 9090 {
		t.Errorf("Expected REST port 9090, got %d", loadedConfig.Server.REST.Port)
	}
}
