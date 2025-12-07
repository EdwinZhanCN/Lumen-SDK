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

// New tests for chunk config defaults and presets

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

func TestPresetChunkConfigs(t *testing.T) {
	// Test minimal preset
	minimalCfg, err := config2.PresetConfig("minimal")
	if err != nil {
		t.Fatalf("PresetConfig(minimal) error = %v", err)
	}
	if !minimalCfg.Chunk.EnableAuto {
		t.Error("Minimal preset should have chunking enabled")
	}
	if minimalCfg.Chunk.Threshold != 256*1024 {
		t.Errorf("Minimal preset expected Threshold %d, got %d", 256*1024, minimalCfg.Chunk.Threshold)
	}
	if minimalCfg.Chunk.MaxChunkBytes != 64*1024 {
		t.Errorf("Minimal preset expected MaxChunkBytes %d, got %d", 64*1024, minimalCfg.Chunk.MaxChunkBytes)
	}

	// Test lightweight preset
	lightweightCfg, err := config2.PresetConfig("lightweight")
	if err != nil {
		t.Fatalf("PresetConfig(lightweight) error = %v", err)
	}
	if !lightweightCfg.Chunk.EnableAuto {
		t.Error("Lightweight preset should have chunking enabled")
	}
	if lightweightCfg.Chunk.Threshold != 512*1024 {
		t.Errorf("Lightweight preset expected Threshold %d, got %d", 512*1024, lightweightCfg.Chunk.Threshold)
	}
	if lightweightCfg.Chunk.MaxChunkBytes != 128*1024 {
		t.Errorf("Lightweight preset expected MaxChunkBytes %d, got %d", 128*1024, lightweightCfg.Chunk.MaxChunkBytes)
	}

	// Test brave preset
	braveCfg, err := config2.PresetConfig("brave")
	if err != nil {
		t.Fatalf("PresetConfig(brave) error = %v", err)
	}
	if !braveCfg.Chunk.EnableAuto {
		t.Error("Brave preset should have chunking enabled")
	}
	if braveCfg.Chunk.Threshold != 4<<20 {
		t.Errorf("Brave preset expected Threshold %d, got %d", 4<<20, braveCfg.Chunk.Threshold)
	}
	if braveCfg.Chunk.MaxChunkBytes != 1<<20 {
		t.Errorf("Brave preset expected MaxChunkBytes %d, got %d", 1<<20, braveCfg.Chunk.MaxChunkBytes)
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

func TestValidateWithErrors(t *testing.T) {
	config := &config2.Config{
		Discovery: config2.DiscoveryConfig{
			Enabled:     true,
			ServiceType: "", // 无效
		},
		Server: config2.ServerConfig{
			REST: config2.RESTConfig{
				Enabled: true,
				Port:    0, // 无效
			},
		},
		Logging: config2.LoggingConfig{
			Level: "invalid", // 无效
		},
	}

	errors := config.ValidateWithErrors()
	if len(errors) == 0 {
		t.Error("Expected validation errors, got none")
	}

	// 检查具体错误
	errorFields := make(map[string]bool)
	for _, err := range errors {
		if configErr, ok := err.(*config2.ConfigError); ok {
			errorFields[configErr.Field] = true
		}
	}

	expectedFields := []string{
		"discovery.service_type",
		"server.rest.port",
		"logging.level",
	}

	for _, field := range expectedFields {
		if !errorFields[field] {
			t.Errorf("Expected error for field '%s', but not found", field)
		}
	}
}

// New tests to assert chunk-related validation behavior
func TestChunkValidationErrors(t *testing.T) {
	// Case 1: negative threshold
	cfg1 := config2.DefaultConfig()
	cfg1.Chunk.Threshold = -1
	errs := cfg1.ValidateWithErrors()
	found := false
	for _, e := range errs {
		if ce, ok := e.(*config2.ConfigError); ok && ce.Field == "chunk.threshold" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected validation error for chunk.threshold negative value")
	}

	// Case 2: MaxChunkBytes unreasonably large
	cfg2 := config2.DefaultConfig()
	cfg2.Chunk.EnableAuto = true
	cfg2.Connection.MaxMessageSize = 0   // allow larger than connection for this test path
	cfg2.Chunk.MaxChunkBytes = 200 << 20 // 200 MiB -> should trigger too large error
	errs2 := cfg2.ValidateWithErrors()
	foundLarge := false
	for _, e := range errs2 {
		if ce, ok := e.(*config2.ConfigError); ok && ce.Field == "chunk.max_chunk_bytes" {
			foundLarge = true
			break
		}
	}
	if !foundLarge {
		t.Error("Expected validation error for chunk.max_chunk_bytes being unreasonably large")
	}

	// Case 3: MaxChunkBytes greater than Threshold
	cfg3 := config2.DefaultConfig()
	cfg3.Chunk.Threshold = 100 * 1024     // 100 KiB
	cfg3.Chunk.MaxChunkBytes = 200 * 1024 // 200 KiB > threshold
	errs3 := cfg3.ValidateWithErrors()
	foundRelation := false
	for _, e := range errs3 {
		if ce, ok := e.(*config2.ConfigError); ok && ce.Field == "chunk.max_chunk_bytes" {
			foundRelation = true
			break
		}
	}
	if !foundRelation {
		t.Error("Expected validation error when max_chunk_bytes > threshold")
	}
}
