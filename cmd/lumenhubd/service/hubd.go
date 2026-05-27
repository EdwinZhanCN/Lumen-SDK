package service

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/edwinzhancn/lumen-sdk/cmd/lumenhubd/internal"
	"github.com/edwinzhancn/lumen-sdk/pkg/client"
	"github.com/edwinzhancn/lumen-sdk/pkg/config"
	"github.com/edwinzhancn/lumen-sdk/pkg/discovery"
	"github.com/edwinzhancn/lumen-sdk/pkg/server/rest"

	ws "github.com/gofiber/contrib/websocket"
	"go.uber.org/zap"
)

// HubdService manages the hubd service lifecycle
type HubdService struct {
	config          *config.Config
	logger          *zap.Logger
	client          *client.LumenClient
	servers         []func() error
	serverShutdowns []func() error
	startTime       time.Time

	// WebSocket node watch
	wsClients map[*ws.Conn]struct{}
	wsMu      sync.Mutex
	prevNodes map[string]*discovery.NodeInfo
}

// NewHubdService creates a new hubd service instance
func NewHubdService(cfg *config.Config, logger *zap.Logger) (*HubdService, error) {
	return &HubdService{
		config:    cfg,
		logger:    logger,
		wsClients: make(map[*ws.Conn]struct{}),
		prevNodes: make(map[string]*discovery.NodeInfo),
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
	_ = ctx

	// Clear server lifecycle hooks.
	s.servers = []func() error{}
	s.serverShutdowns = []func() error{}

	// Start REST server if enabled.
	if s.config.Server.REST.Enabled {
		handler := rest.NewHandler(s.client, nil, s.logger)
		router := rest.NewRouter(handler, s.logger)
		router.SetupRoutes()

		// WebSocket endpoint for node watch (push-based discovery).
		router.App().Get("/v1/nodes/watch", ws.New(s.handleNodeWatch))

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

// Stop stops the hubd service gracefully
func (s *HubdService) Stop() error {
	s.logger.Info("Stopping Lumen Hub service...")

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

func countActiveNodes(nodes []*discovery.NodeInfo) int {
	count := 0
	for _, node := range nodes {
		if node.IsActive() {
			count++
		}
	}
	return count
}

// ---- WebSocket Node Watch ----

// handleNodeWatch is the WebSocket handler for /v1/nodes/watch.
// It sends the current node snapshot followed by incremental diffs.
func (s *HubdService) handleNodeWatch(conn *ws.Conn) {
	s.wsMu.Lock()
	s.wsClients[conn] = struct{}{}
	s.wsMu.Unlock()

	defer func() {
		s.wsMu.Lock()
		delete(s.wsClients, conn)
		s.wsMu.Unlock()
	}()

	// Send current snapshot.
	if s.client != nil {
		nodes := s.client.GetNodes()
		if len(nodes) > 0 {
			snapshot := nodeSnapshotMsg(nodes)
			if err := conn.WriteJSON(snapshot); err != nil {
				s.logger.Debug("ws snapshot write failed", zap.Error(err))
				return
			}
		}
	}

	s.startNodeWatcher()

	// Keep connection alive; the watcher pushes diffs via broadcastNodeDiff.
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (s *HubdService) startNodeWatcher() {
	if s.client == nil {
		return
	}
	s.logger.Info("starting node watcher for WebSocket push")

	s.client.WatchNodes(func(nodes []*discovery.NodeInfo) {
		s.broadcastNodeDiff(nodes)
	})
}

func (s *HubdService) broadcastNodeDiff(nodes []*discovery.NodeInfo) {
	s.wsMu.Lock()
	clients := make([]*ws.Conn, 0, len(s.wsClients))
	for c := range s.wsClients {
		clients = append(clients, c)
	}
	s.wsMu.Unlock()

	if len(clients) == 0 {
		return
	}

	current := make(map[string]*discovery.NodeInfo, len(nodes))
	for _, n := range nodes {
		if n != nil && n.IsActive() {
			current[n.ID] = n
		}
	}

	// Find added nodes.
	for id, node := range current {
		if _, ok := s.prevNodes[id]; !ok {
			s.logger.Debug("ws push: node added", zap.String("id", id))
			s.broadcastEvent(clients, nodeAddedMsg(node))
		}
	}

	// Find removed nodes.
	for id := range s.prevNodes {
		if _, ok := current[id]; !ok {
			s.logger.Debug("ws push: node removed", zap.String("id", id))
			s.broadcastEvent(clients, nodeRemovedMsg(id))
		}
	}

	s.prevNodes = current
}

func (s *HubdService) broadcastEvent(clients []*ws.Conn, msg interface{}) {
	for _, c := range clients {
		if err := c.WriteJSON(msg); err != nil {
			s.logger.Debug("ws event write failed", zap.Error(err))
		}
	}
}

// ---- WebSocket JSON messages ----

type wsNodeInfo struct {
	NodeID  string   `json:"node_id"`
	Address string   `json:"address"`
	Tasks   []string `json:"tasks,omitempty"`
}

type wsNodeEvent struct {
	Type   string       `json:"type"` // "snapshot", "added", "removed"
	Nodes  []wsNodeInfo `json:"nodes,omitempty"`
	Node   *wsNodeInfo  `json:"node,omitempty"`
	NodeID string       `json:"node_id,omitempty"`
}

func nodeSnapshotMsg(nodes []*discovery.NodeInfo) wsNodeEvent {
	infos := make([]wsNodeInfo, 0, len(nodes))
	for _, n := range nodes {
		if n == nil || !n.IsActive() {
			continue
		}
		tasks := make([]string, 0, len(n.Tasks))
		for _, t := range n.Tasks {
			tasks = append(tasks, t.Name)
		}
		infos = append(infos, wsNodeInfo{
			NodeID:  n.ID,
			Address: n.Address,
			Tasks:   tasks,
		})
	}
	return wsNodeEvent{Type: "snapshot", Nodes: infos}
}

func nodeAddedMsg(n *discovery.NodeInfo) wsNodeEvent {
	tasks := make([]string, 0, len(n.Tasks))
	for _, t := range n.Tasks {
		tasks = append(tasks, t.Name)
	}
	return wsNodeEvent{
		Type: "added",
		Node: &wsNodeInfo{
			NodeID:  n.ID,
			Address: n.Address,
			Tasks:   tasks,
		},
	}
}

func nodeRemovedMsg(nodeID string) wsNodeEvent {
	return wsNodeEvent{Type: "removed", NodeID: nodeID}
}
