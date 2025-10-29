package internal

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/edwinzhancn/lumen-sdk/pkg/client"
	"github.com/edwinzhancn/lumen-sdk/pkg/config"

	"go.uber.org/zap"
)

var (
	globalClient *client.LumenClient
	globalLogger *zap.Logger
	globalConfig *config.Config
	startTime    time.Time
	mu           sync.RWMutex
)

// InitializeClient initializes the global client instance
func InitializeClient(cfg *config.Config, logger *zap.Logger) error {
	mu.Lock()
	defer mu.Unlock()

	if globalClient != nil {
		return fmt.Errorf("client already initialized")
	}

	// Create client
	lumenClient, err := client.NewLumenClient(cfg, logger)
	if err != nil {
		return fmt.Errorf("failed to create Lumen client: %w", err)
	}

	// Start client
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := lumenClient.Start(ctx); err != nil {
		return fmt.Errorf("failed to start Lumen client: %w", err)
	}

	globalClient = lumenClient
	globalLogger = logger
	globalConfig = cfg
	startTime = time.Now()

	return nil
}

// GetClient returns the global client instance
func GetClient() (*client.LumenClient, error) {
	mu.RLock()
	defer mu.RUnlock()

	if globalClient == nil {
		return nil, fmt.Errorf("client not initialized")
	}

	return globalClient, nil
}

// GetLogger returns the global logger instance
func GetLogger() (*zap.Logger, error) {
	mu.RLock()
	defer mu.RUnlock()

	if globalLogger == nil {
		return nil, fmt.Errorf("logger not initialized")
	}

	return globalLogger, nil
}

// GetConfig returns the global config instance
func GetConfig() (*config.Config, error) {
	mu.RLock()
	defer mu.RUnlock()

	if globalConfig == nil {
		return nil, fmt.Errorf("config not initialized")
	}

	return globalConfig, nil
}

// GetStartTime returns when the hub was started
func GetStartTime() time.Time {
	mu.RLock()
	defer mu.RUnlock()
	return startTime
}

// IsInitialized returns whether the global client is initialized
func IsInitialized() bool {
	mu.RLock()
	defer mu.RUnlock()
	return globalClient != nil
}

// CloseClient closes the global client instance
func CloseClient() error {
	mu.Lock()
	defer mu.Unlock()

	if globalClient == nil {
		return nil
	}

	if err := globalClient.Close(); err != nil {
		return fmt.Errorf("failed to close client: %w", err)
	}

	// Sync logger before closing
	if globalLogger != nil {
		globalLogger.Sync()
	}

	globalClient = nil
	globalLogger = nil
	globalConfig = nil

	return nil
}

// ResetClient resets the global client instance (for testing)
func ResetClient() {
	mu.Lock()
	defer mu.Unlock()

	globalClient = nil
	globalLogger = nil
	globalConfig = nil
	startTime = time.Time{}
}
