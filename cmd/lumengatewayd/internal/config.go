package internal

import (
	"fmt"

	"github.com/edwinzhancn/lumen-sdk/pkg/config"
)

// LoadConfig loads configuration from the provided config file path.
// If cfgFile is empty, the default configuration is used.
// Environment variable overrides are applied after loading.
func LoadConfig(cfgFile string) (*config.Config, error) {
	cfg, err := config.LoadConfig(cfgFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	return cfg, nil
}
