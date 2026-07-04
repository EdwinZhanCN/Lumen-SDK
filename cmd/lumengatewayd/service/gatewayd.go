package service

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/edwinzhancn/lumen-sdk/cmd/lumengatewayd/internal"
	"github.com/edwinzhancn/lumen-sdk/pkg/client"
	"github.com/edwinzhancn/lumen-sdk/pkg/config"
	"github.com/edwinzhancn/lumen-sdk/pkg/discovery"
	"github.com/edwinzhancn/lumen-sdk/pkg/server/rest"

	"go.uber.org/zap"
)

// GatewaydService manages the gatewayd service lifecycle
type GatewaydService struct {
	config          *config.Config
	logger          *zap.Logger
	client          *client.LumenClient
	servers         []func() error
	serverShutdowns []func() error
	startTime       time.Time
}

// NewGatewaydService creates a new gatewayd service instance
func NewGatewaydService(cfg *config.Config, logger *zap.Logger) (*GatewaydService, error) {
	return &GatewaydService{
		config: cfg,
		logger: logger,
	}, nil
}

// Start starts the gatewayd service
func (s *GatewaydService) Start(ctx context.Context) error {
	s.logger.Info("Starting Lumen Gateway service...")

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
	s.logger.Info("Lumen Gateway service started successfully",
		zap.String("version", "1.0.0"),
		zap.Int("servers", len(s.servers)))

	return nil
}

// startServers starts all configured servers
func (s *GatewaydService) startServers(ctx context.Context) error {
	_ = ctx

	// Clear server lifecycle hooks.
	s.servers = []func() error{}
	s.serverShutdowns = []func() error{}

	// Start REST server if enabled. The route set includes the
	// /v1/nodes/watch push-discovery WebSocket (shared rest package).
	if s.config.Server.REST.Enabled {
		handler := rest.NewHandler(s.client, nil, s.logger)
		router := rest.NewRouter(handler, s.logger)
		router.SetupRoutes()

		addr := fmt.Sprintf("%s:%d", s.config.Server.REST.Host, s.config.Server.REST.Port)
		s.servers = append(s.servers, func() error {
			s.logger.Info("Starting REST server", zap.String("address", addr))
			return router.Start(addr)
		})
		s.serverShutdowns = append(s.serverShutdowns, func() error {
			return router.ShutdownWithTimeout(5 * time.Second)
		})
	}

	// Start all servers in goroutines.
	for i, startServer := range s.servers {
		go func(index int, startFn func() error) {
			if err := startFn(); err != nil {
				s.logger.Error("Server stopped with error",
					zap.Int("server_index", index),
					zap.Error(err))
			}
		}(i, startServer)
	}

	return nil
}

// Stop stops the gatewayd service gracefully
func (s *GatewaydService) Stop() error {
	s.logger.Info("Stopping Lumen Gateway service...")

	for i := len(s.serverShutdowns) - 1; i >= 0; i-- {
		if err := s.serverShutdowns[i](); err != nil {
			s.logger.Error("Failed to stop server", zap.Int("server_index", i), zap.Error(err))
		}
	}
	s.serverShutdowns = nil

	// Close the global client through internal.CloseClient so its lifecycle context
	// is cancelled before client subsystems are stopped.
	if err := internal.CloseClient(); err != nil {
		s.logger.Error("Failed to close internal client", zap.Error(err))
	}
	s.client = nil
	s.startTime = time.Time{}

	s.logger.Info("Lumen Gateway service stopped")
	return nil
}

// WaitForShutdown waits for shutdown signals
func (s *GatewaydService) WaitForShutdown() {
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
func (s *GatewaydService) GetUptime() time.Duration {
	if s.startTime.IsZero() {
		return 0
	}
	return time.Since(s.startTime)
}

// IsRunning returns whether the service is running
func (s *GatewaydService) IsRunning() bool {
	return !s.startTime.IsZero()
}

// GetStatus returns the current service status
func (s *GatewaydService) GetStatus() map[string]interface{} {
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

func countActiveNodes(nodes []*discovery.NodeInfo) int {
	count := 0
	for _, node := range nodes {
		if node.IsActive() {
			count++
		}
	}
	return count
}

// The /v1/nodes/watch WebSocket implementation lives in pkg/server/rest
// (node_watch.go) and is registered by rest.SetupRoutes, so gatewayd and
// lumen-gateway share it.
