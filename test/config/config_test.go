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
	os.Setenv("LUMEN_REST_HOST", "127.0.0.1")
	os.Setenv("LUMEN_REST_PORT", "9090")
	os.Setenv("LUMEN_LOG_LEVEL", "debug")
	os.Setenv("LUMEN_LOG_FORMAT", "text")

	defer func() {
		// 清理环境变量
		os.Unsetenv("LUMEN_DISCOVERY_ENABLED")
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
