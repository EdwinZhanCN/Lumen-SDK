package config

import "time"

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		Discovery: DiscoveryConfig{
			Enabled:      true,
			ServiceType:  "_lumen._tcp",
			Domain:       "local",
			ScanInterval: 30 * time.Second,
			NodeTimeout:  5 * time.Minute,
			MDNSEnabled:  true,
			HubURL:       "",
		},
		Server: ServerConfig{
			REST: RESTConfig{
				Enabled: true,
				Host:    "0.0.0.0",
				Port:    5866,
				CORS:    true,
			},
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		},
		Chunk: ChunkConfig{
			EnableAuto:    true,
			Threshold:     1 << 20,    // 1 MiB
			MaxChunkBytes: 256 * 1024, // 256 KiB
		},
	}
}
