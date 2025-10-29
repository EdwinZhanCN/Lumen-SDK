package internal

import (
	"fmt"

	"github.com/edwinzhancn/lumen-sdk/pkg/config"
)

// LoadPresetConfig loads a built-in preset configuration
func LoadPresetConfig(preset string) (*config.Config, error) {
	if preset == "" {
		return nil, fmt.Errorf("preset name cannot be empty")
	}

	cfg, err := config.PresetConfig(preset)
	if err != nil {
		return nil, fmt.Errorf("failed to load preset '%s': %w", preset, err)
	}

	return cfg, nil
}

// LoadConfig loads configuration based on the provided parameters
func LoadConfig(cfgFile, preset string) (*config.Config, error) {
	var cfg *config.Config
	var err error

	// Determine configuration source
	if cfgFile != "" {
		// Load from explicit config file
		cfg, err = config.LoadConfig(cfgFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load config from %s: %w", cfgFile, err)
		}
	} else if preset != "" {
		// Load from built-in preset
		cfg, err = LoadPresetConfig(preset)
		if err != nil {
			return nil, err
		}
	} else {
		// Use default configuration
		cfg = config.DefaultConfig()
	}

	// Load environment variables (overwrites file/preset values)
	if err := cfg.LoadFromEnv(); err != nil {
		return nil, fmt.Errorf("failed to load environment variables: %w", err)
	}

	// Validate final configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}
