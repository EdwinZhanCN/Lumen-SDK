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
	globalCancel context.CancelFunc
	startTime    time.Time
	mu           sync.RWMutex
)

// InitializeClient initializes the global discovery client the Host Broker
// uses internally to aggregate node events before republishing them over
// /v1/nodes/watch.
//
// The internal client's Broker URL is always forced empty, regardless of
// what cfg sets: the Broker is the thing other processes subscribe to, so if
// it also subscribed to a Broker URL — especially its own — that would set
// up a self-subscription loop. mDNS and static nodes pass through unchanged.
func InitializeClient(cfg *config.Config, logger *zap.Logger) error {
	mu.Lock()
	defer mu.Unlock()

	if globalClient != nil {
		return fmt.Errorf("client already initialized")
	}

	internalCfg := *cfg
	internalCfg.Discovery.BrokerURL = ""

	lumenClient, err := client.NewLumenClient(&internalCfg, logger)
	if err != nil {
		return fmt.Errorf("failed to create Lumen client: %w", err)
	}

	// Start with a long-lived lifecycle context. Discovery and connection-pool
	// maintenance run until CloseClient cancels this context.
	ctx, cancel := context.WithCancel(context.Background())

	if err := lumenClient.Start(ctx); err != nil {
		cancel()
		return fmt.Errorf("failed to start Lumen client: %w", err)
	}

	globalClient = lumenClient
	globalLogger = logger
	globalConfig = cfg
	globalCancel = cancel
	startTime = time.Now()

	return nil
}

// GetClient returns the global client instance.
func GetClient() (*client.LumenClient, error) {
	mu.RLock()
	defer mu.RUnlock()

	if globalClient == nil {
		return nil, fmt.Errorf("client not initialized")
	}

	return globalClient, nil
}

// GetLogger returns the global logger instance.
func GetLogger() (*zap.Logger, error) {
	mu.RLock()
	defer mu.RUnlock()

	if globalLogger == nil {
		return nil, fmt.Errorf("logger not initialized")
	}

	return globalLogger, nil
}

// GetConfig returns the global config instance (the caller's original
// config, not the Broker-URL-stripped copy used internally).
func GetConfig() (*config.Config, error) {
	mu.RLock()
	defer mu.RUnlock()

	if globalConfig == nil {
		return nil, fmt.Errorf("config not initialized")
	}

	return globalConfig, nil
}

// GetStartTime returns when the client was started.
func GetStartTime() time.Time {
	mu.RLock()
	defer mu.RUnlock()
	return startTime
}

// IsInitialized returns whether the global client is initialized.
func IsInitialized() bool {
	mu.RLock()
	defer mu.RUnlock()
	return globalClient != nil
}

// CloseClient closes the global client instance.
func CloseClient() error {
	mu.Lock()
	defer mu.Unlock()

	if globalClient == nil {
		return nil
	}

	if globalCancel != nil {
		globalCancel()
		globalCancel = nil
	}

	if err := globalClient.Close(); err != nil {
		return fmt.Errorf("failed to close client: %w", err)
	}

	if globalLogger != nil {
		globalLogger.Sync()
	}

	globalClient = nil
	globalLogger = nil
	globalConfig = nil
	globalCancel = nil

	return nil
}

// ResetClient resets the global client instance (for testing).
func ResetClient() {
	mu.Lock()
	defer mu.Unlock()

	if globalCancel != nil {
		globalCancel()
	}
	globalClient = nil
	globalLogger = nil
	globalConfig = nil
	globalCancel = nil
	startTime = time.Time{}
}
