package service

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"Lumen-SDK/cmd/lumenhubd/internal"
	"Lumen-SDK/pkg/client"
	"Lumen-SDK/pkg/codec"
	"Lumen-SDK/pkg/config"
	"Lumen-SDK/pkg/server/rest"

	"go.uber.org/zap"
)

// HubdService manages the hubd service lifecycle
type HubdService struct {
	config    *config.Config
	logger    *zap.Logger
	client    *client.LumenClient
	servers   []func() error
	startTime time.Time
}

// NewHubdService creates a new hubd service instance
func NewHubdService(cfg *config.Config, logger *zap.Logger) (*HubdService, error) {
	return &HubdService{
		config: cfg,
		logger: logger,
	}, nil
}

// Start starts the hubd service
func (s *HubdService) Start(ctx context.Context) error {
	s.logger.Info("Starting Lumen Hub service...")

	// Initialize the global client
	if err := internal.InitializeClient(s.config, s.logger); err != nil {
		return fmt.Errorf("failed to initialize client: %w", err)
	}

	// Get the global client
	client, err := internal.GetClient()
	if err != nil {
		return fmt.Errorf("failed to get client: %w", err)
	}
	s.client = client

	// Start servers
	if err := s.startServers(ctx); err != nil {
		return fmt.Errorf("failed to start servers: %w", err)
	}

	s.startTime = time.Now()
	s.logger.Info("Lumen Hub service started successfully",
		zap.String("version", "1.0.0"),
		zap.Int("servers", len(s.servers)))

	return nil
}

// startServers starts all configured servers
func (s *HubdService) startServers(ctx context.Context) error {
	// Clear servers slice
	s.servers = []func() error{}

	// Start REST server if enabled
	if s.config.Server.REST.Enabled {
		s.servers = append(s.servers, func() error {
			return s.startRESTServer()
		})
	}

	// Start all servers in goroutines
	for i, startServer := range s.servers {
		go func(index int, startFn func() error) {
			if err := startFn(); err != nil {
				s.logger.Error("Server failed to start",
					zap.Int("server_index", index),
					zap.Error(err))
			}
		}(i, startServer)
	}

	return nil
}

// startRESTServer starts the REST API server
func (s *HubdService) startRESTServer() error {
	codecRegistry := codec.GetDefaultRegistry()
	handler := rest.NewHandler(s.client, codecRegistry, s.logger)
	router := rest.NewRouter(handler, s.logger)

	router.SetupRoutes()

	addr := fmt.Sprintf("%s:%d", s.config.Server.REST.Host, s.config.Server.REST.Port)
	s.logger.Info("Starting REST server", zap.String("address", addr))

	return router.Start(addr)
}

// Stop stops the hubd service gracefully
func (s *HubdService) Stop() error {
	s.logger.Info("Stopping Lumen Hub service...")

	// Close client
	if s.client != nil {
		if err := s.client.Close(); err != nil {
			s.logger.Error("Failed to close client", zap.Error(err))
		}
	}

	// Close internal client
	if err := internal.CloseClient(); err != nil {
		s.logger.Error("Failed to close internal client", zap.Error(err))
	}

	s.logger.Info("Lumen Hub service stopped")
	return nil
}

// WaitForShutdown waits for shutdown signals
func (s *HubdService) WaitForShutdown() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	s.logger.Info("Waiting for shutdown signal...")

	sig := <-sigCh
	s.logger.Info("Received shutdown signal", zap.String("signal", sig.String()))

	// Stop service
	if err := s.Stop(); err != nil {
		s.logger.Error("Error stopping service", zap.Error(err))
	}
}

// GetUptime returns the service uptime
func (s *HubdService) GetUptime() time.Duration {
	if s.startTime.IsZero() {
		return 0
	}
	return time.Since(s.startTime)
}

// IsRunning returns whether the service is running
func (s *HubdService) IsRunning() bool {
	return !s.startTime.IsZero()
}

// GetStatus returns the current service status
func (s *HubdService) GetStatus() map[string]interface{} {
	status := map[string]interface{}{
		"version":   "1.0.0",
		"running":   s.IsRunning(),
		"uptime":    s.GetUptime().String(),
		"timestamp": time.Now(),
	}

	if s.client != nil {
		nodes := s.client.GetNodes()
		status["nodes"] = map[string]interface{}{
			"total":  len(nodes),
			"active": countActiveNodes(nodes),
		}
	}

	return status
}

func countActiveNodes(nodes []*client.NodeInfo) int {
	count := 0
	for _, node := range nodes {
		if node.IsActive() {
			count++
		}
	}
	return count
}
