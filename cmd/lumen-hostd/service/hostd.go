package service

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/edwinzhancn/lumen-sdk/cmd/lumen-hostd/internal"
	"github.com/edwinzhancn/lumen-sdk/pkg/client"
	"github.com/edwinzhancn/lumen-sdk/pkg/config"
	"github.com/edwinzhancn/lumen-sdk/pkg/discovery"
	"github.com/edwinzhancn/lumen-sdk/pkg/hostbroker"

	"go.uber.org/zap"
)

// BuildInfo carries ldflags-injected version metadata, set by main and
// surfaced at GET /v1/version.
type BuildInfo struct {
	Version   string
	Commit    string
	BuildTime string
}

// HostdService manages the Host Broker daemon's lifecycle: an internal
// discovery client that aggregates mDNS/static node events, republished over
// a discovery-only pkg/hostbroker server. It never serves inference.
type HostdService struct {
	config    *config.Config
	logger    *zap.Logger
	build     BuildInfo
	client    *client.LumenClient
	broker    *hostbroker.Server
	startTime time.Time
}

// NewHostdService creates a new Host Broker service instance.
func NewHostdService(cfg *config.Config, build BuildInfo, logger *zap.Logger) (*HostdService, error) {
	return &HostdService{
		config: cfg,
		build:  build,
		logger: logger,
	}, nil
}

// Start starts the Host Broker service.
func (s *HostdService) Start(ctx context.Context) error {
	s.logger.Info("Starting Lumen Host Broker...")

	if err := internal.InitializeClient(s.config, s.logger); err != nil {
		return fmt.Errorf("failed to initialize client: %w", err)
	}

	lumenClient, err := internal.GetClient()
	if err != nil {
		return fmt.Errorf("failed to get client: %w", err)
	}
	s.client = lumenClient

	if err := s.startBroker(ctx); err != nil {
		return fmt.Errorf("failed to start broker server: %w", err)
	}

	s.startTime = time.Now()
	s.logger.Info("Lumen Host Broker started successfully",
		zap.String("version", s.build.Version))

	return nil
}

func (s *HostdService) startBroker(ctx context.Context) error {
	_ = ctx

	if !s.config.Server.REST.Enabled {
		return nil
	}

	version := hostbroker.VersionInfo{
		Version:   s.build.Version,
		Commit:    s.build.Commit,
		BuildTime: s.build.BuildTime,
	}
	broker := hostbroker.NewServer(s.client, version, s.logger)
	s.broker = broker

	// The goroutine below closes over the local broker variable, not
	// s.broker: if Stop() runs before this goroutine is scheduled, it nils
	// out s.broker, and reading that (rather than the stable local) here
	// would nil-pointer panic on broker.Start.
	addr := fmt.Sprintf("%s:%d", s.config.Server.REST.Host, s.config.Server.REST.Port)
	go func() {
		if err := broker.Start(addr); err != nil {
			s.logger.Error("Broker server stopped with error", zap.Error(err))
		}
	}()

	return nil
}

// Stop stops the Host Broker service gracefully.
func (s *HostdService) Stop() error {
	s.logger.Info("Stopping Lumen Host Broker...")

	if s.broker != nil {
		if err := s.broker.ShutdownWithTimeout(5 * time.Second); err != nil {
			s.logger.Error("Failed to stop broker server", zap.Error(err))
		}
		// ShutdownWithTimeout does not track hijacked connections (the
		// /v1/nodes/watch WebSocket upgrade), so close those separately.
		s.broker.Close()
		s.broker = nil
	}

	if err := internal.CloseClient(); err != nil {
		s.logger.Error("Failed to close internal client", zap.Error(err))
	}
	s.client = nil
	s.startTime = time.Time{}

	s.logger.Info("Lumen Host Broker stopped")
	return nil
}

// WaitForShutdown blocks until a shutdown signal is received, then stops.
func (s *HostdService) WaitForShutdown() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	s.logger.Info("Waiting for shutdown signal...")

	sig := <-sigCh
	s.logger.Info("Received shutdown signal", zap.String("signal", sig.String()))

	if err := s.Stop(); err != nil {
		s.logger.Error("Error stopping service", zap.Error(err))
	}
}

// GetUptime returns the service uptime.
func (s *HostdService) GetUptime() time.Duration {
	if s.startTime.IsZero() {
		return 0
	}
	return time.Since(s.startTime)
}

// IsRunning returns whether the service is running.
func (s *HostdService) IsRunning() bool {
	return !s.startTime.IsZero()
}

// GetStatus returns the current service status.
func (s *HostdService) GetStatus() map[string]interface{} {
	status := map[string]interface{}{
		"version":   s.build.Version,
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
