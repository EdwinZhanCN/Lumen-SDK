package discovery

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/edwinzhancn/lumen-sdk/pkg/config"
	"github.com/edwinzhancn/lumen-sdk/pkg/utils"
	pb "github.com/edwinzhancn/lumen-sdk/proto"

	"github.com/grandcat/zeroconf"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

// MDNSDiscovery mDNS service discovery implementation.
type MDNSDiscovery struct {
	config   *config.DiscoveryConfig
	nodes    map[string]*NodeInfo
	mu       sync.RWMutex
	resolver *zeroconf.Resolver
	watchers []func([]*NodeInfo)
	ctx      context.Context
	cancel   context.CancelFunc
	running  bool
	logger   *zap.Logger
	lastErr  error
}

func NewMDNSDiscovery(cfg *config.DiscoveryConfig, logger *zap.Logger) *MDNSDiscovery {
	if cfg == nil {
		cfg = &config.DiscoveryConfig{
			Enabled:      true,
			ServiceType:  "_lumen._tcp",
			Domain:       "local",
			ScanInterval: 30 * time.Second,
			NodeTimeout:  5 * time.Minute,
			MaxNodes:     20,
		}
	}
	logger = ensureLogger(logger)

	return &MDNSDiscovery{
		config:   cfg,
		nodes:    make(map[string]*NodeInfo),
		watchers: make([]func([]*NodeInfo), 0),
		logger:   logger,
	}
}

func (m *MDNSDiscovery) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.config.Enabled {
		m.logger.Info("mDNS discovery is disabled")
		m.lastErr = nil
		return nil
	}

	if m.running {
		return utils.DiscoveryFailedError("discovery is already running")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if m.cancel != nil {
		m.cancel()
	}
	m.ctx, m.cancel = context.WithCancel(ctx)

	err := utils.SafeExecute(func() error {
		resolver, err := zeroconf.NewResolver(nil)
		if err != nil {
			return utils.ConnectionFailedError("failed to create mDNS resolver",
				map[string]interface{}{
					"error":        err.Error(),
					"service_type": m.config.ServiceType,
					"domain":       m.config.Domain,
				})
		}
		m.resolver = resolver
		return nil
	})
	if err != nil {
		m.lastErr = err
		return utils.Wrap(err, utils.ErrCodeConnectionFailed, "failed to initialize mDNS resolver")
	}

	m.running = true
	m.lastErr = nil

	go m.scanLoop(m.ctx)
	go m.cleanupLoop(m.ctx)

	m.logger.Info("mDNS discovery started",
		zap.String("service_type", m.config.ServiceType),
		zap.String("domain", m.config.Domain),
		zap.Duration("scan_interval", m.config.ScanInterval),
	)
	return nil
}

func (m *MDNSDiscovery) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		m.lastErr = nil
		return nil
	}

	m.logger.Info("stopping mDNS discovery")
	errorAggregator := utils.NewErrorAggregator()

	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	m.ctx = nil

	if m.resolver != nil {
		err := utils.SafeExecute(func() error {
			m.resolver = nil
			return nil
		})
		errorAggregator.Add(err)
	}

	m.running = false
	nodeCount := len(m.nodes)
	m.nodes = make(map[string]*NodeInfo)
	m.watchers = make([]func([]*NodeInfo), 0)

	if errorAggregator.HasErrors() {
		m.lastErr = errorAggregator.ToError()
		m.logger.Error("errors occurred during discovery shutdown",
			zap.Int("error_count", len(errorAggregator.GetErrors())),
			zap.Error(errorAggregator.ToError()))
		return utils.ServiceUnavailableError("discovery shutdown failed", errorAggregator.ToError())
	}

	m.lastErr = nil
	m.logger.Info("mDNS discovery stopped successfully", zap.Int("nodes_cleared", nodeCount))
	return nil
}

func (m *MDNSDiscovery) GetNodes() []*NodeInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	nodes := make([]*NodeInfo, 0, len(m.nodes))
	for _, node := range m.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

func (m *MDNSDiscovery) GetNode(id string) (*NodeInfo, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	node, exists := m.nodes[id]
	return node, exists
}

func (m *MDNSDiscovery) Lookup(ctx context.Context, nodeID string) ([]string, error) {
	_ = ctx
	node, exists := m.GetNode(nodeID)
	if !exists || node == nil || strings.TrimSpace(node.Address) == "" {
		return nil, nil
	}
	return []string{node.Address}, nil
}

func (m *MDNSDiscovery) Error() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastErr
}

func (m *MDNSDiscovery) String() string {
	return "mdns"
}

func (m *MDNSDiscovery) Cache() map[string]CacheEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cache := make(map[string]CacheEntry, len(m.nodes))
	for id, node := range m.nodes {
		if node == nil {
			continue
		}
		cache[id] = CacheEntry{
			Addresses: UniqueTrimmedStrings([]string{node.Address}),
			When:      node.LastSeen,
			Found:     strings.TrimSpace(node.Address) != "",
		}
	}
	return cache
}

func (m *MDNSDiscovery) Watch(callback func([]*NodeInfo)) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.watchers = append(m.watchers, callback)

	if len(m.nodes) > 0 {
		nodes := make([]*NodeInfo, 0, len(m.nodes))
		for _, node := range m.nodes {
			nodes = append(nodes, node)
		}
		go callback(nodes)
	}
	return nil
}

func (m *MDNSDiscovery) scanLoop(ctx context.Context) {
	ticker := time.NewTicker(m.config.ScanInterval)
	defer ticker.Stop()

	m.performScanWithRetry(ctx)
	circuitBreaker := utils.NewCircuitBreaker("mdns-scan", 5, 2*time.Minute)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			err := circuitBreaker.Execute(func() error {
				return m.performScanWithRetry(ctx)
			})
			if err != nil {
				m.logger.Error("scan failed due to circuit breaker",
					zap.String("breaker_state", fmt.Sprintf("%v", circuitBreaker.GetState())),
					zap.Error(err))
				m.mu.Lock()
				m.lastErr = err
				m.mu.Unlock()
				continue
			}
			m.mu.Lock()
			m.lastErr = nil
			m.mu.Unlock()
		}
	}
}

func (m *MDNSDiscovery) performScanWithRetry(ctx context.Context) error {
	retryConfig := &utils.RetryConfig{
		Enabled:     true,
		MaxAttempts: 3,
		Backoff:     500 * time.Millisecond,
		MaxBackoff:  10 * time.Second,
		Multiplier:  1.5,
	}

	retryCallback := func(attempt int, err error) {
		m.logger.Warn("scan retry attempt", zap.Int("attempt", attempt), zap.Error(err))
	}

	return utils.RetryWithCallback(ctx, retryConfig, func(ctx context.Context) error {
		err := m.scanServices()
		if err != nil {
			if strings.Contains(err.Error(), "timeout") ||
				strings.Contains(err.Error(), "connection") ||
				strings.Contains(err.Error(), "resolver") {
				return utils.NewRetryableError(utils.DiscoveryFailedError(err.Error()), true)
			}
			return utils.DiscoveryFailedError(err.Error())
		}
		return nil
	}, retryCallback)
}

func (m *MDNSDiscovery) scanServices() error {
	if !m.running {
		return utils.DiscoveryFailedError("discovery is not running")
	}

	m.logger.Debug("scanning for Lumen services")

	baseCtx := m.ctx
	if baseCtx == nil {
		baseCtx = context.Background()
	}
	scanCtx, cancel := context.WithTimeout(baseCtx, 10*time.Second)
	defer cancel()

	entries := make(chan *zeroconf.ServiceEntry)
	var scanErr error

	err := m.resolver.Lookup(scanCtx, "", m.config.ServiceType, m.config.Domain, entries)
	if err != nil {
		return utils.DiscoveryFailedError("mDNS lookup failed",
			map[string]interface{}{
				"error":        err.Error(),
				"service_type": m.config.ServiceType,
				"domain":       m.config.Domain,
			})
	}

	for {
		select {
		case entry, ok := <-entries:
			if !ok {
				goto DONE
			}
			if entry != nil {
				m.processServiceEntry(entry)
			}
		case <-scanCtx.Done():
			scanErr = utils.TimeoutError("mDNS scan timeout",
				map[string]interface{}{
					"timeout":      scanCtx.Err().Error(),
					"service_type": m.config.ServiceType,
				})
			goto DONE
		}
	}

DONE:
	if scanErr != nil {
		m.logger.Warn("service scan completed with errors", zap.Error(scanErr))
		return scanErr
	}

	m.logger.Debug("service scan completed successfully")
	return nil
}

func (m *MDNSDiscovery) processServiceEntry(entry *zeroconf.ServiceEntry) {
	m.logger.Debug("found service entry",
		zap.String("service_name", entry.Service),
		zap.String("instance_name", entry.Instance),
		zap.Int("addr_count", len(entry.AddrIPv4)),
	)

	for _, addr := range entry.AddrIPv4 {
		if entry.Port == 0 {
			continue
		}

		address := fmt.Sprintf("%s:%d", addr.String(), entry.Port)
		nodeID := m.generateNodeID(entry.Instance, address)

		m.mu.Lock()
		if len(m.nodes) >= m.config.MaxNodes {
			if _, exists := m.nodes[nodeID]; !exists {
				m.mu.Unlock()
				m.logger.Warn("maximum node limit reached, ignoring new node",
					zap.String("node_id", nodeID),
					zap.Int("max_nodes", m.config.MaxNodes))
				continue
			}
		}

		node, exists := m.nodes[nodeID]
		if !exists {
			node = &NodeInfo{
				ID:       nodeID,
				Name:     entry.Instance,
				Address:  address,
				Status:   NodeStatusStarting,
				Metadata: make(map[string]interface{}),
			}

			for _, txt := range entry.Text {
				if kv := strings.SplitN(txt, "=", 2); len(kv) == 2 {
					node.Metadata[kv[0]] = kv[1]
				}
			}

			m.nodes[nodeID] = node

			m.logger.Info("discovered new node",
				zap.String("node_id", nodeID),
				zap.String("name", node.Name),
				zap.String("address", node.Address))
		}

		node.LastSeen = time.Now()
		if node.Status == NodeStatusStarting {
			node.Status = NodeStatusActive
		}
		m.mu.Unlock()

		go m.fetchNodeCapabilities(node)
	}

	m.notifyWatchers()
}

func (m *MDNSDiscovery) fetchNodeCapabilities(node *NodeInfo) {
	m.mu.RLock()
	running := m.running
	baseCtx := m.ctx
	m.mu.RUnlock()
	if !running {
		return
	}
	if baseCtx == nil {
		baseCtx = context.Background()
	}

	m.logger.Debug("fetching capabilities for node", zap.String("node_id", node.ID))

	conn, err := m.connectToNode(node.Address)
	if err != nil {
		m.logger.Error("failed to connect to node", zap.String("node_id", node.ID), zap.Error(err))
		m.mu.Lock()
		node.Status = NodeStatusError
		m.mu.Unlock()
		return
	}
	defer conn.Close()

	client := pb.NewInferenceClient(conn)
	ctx, cancel := context.WithTimeout(baseCtx, 10*time.Second)
	defer cancel()

	capability, err := client.GetCapabilities(ctx, &emptypb.Empty{})
	if err != nil {
		m.logger.Error("failed to get capabilities from node", zap.String("node_id", node.ID), zap.Error(err))
		m.mu.Lock()
		node.Status = NodeStatusError
		m.mu.Unlock()
		return
	}

	m.applyCapability(node, capability)

	m.logger.Info("successfully fetched capabilities for node",
		zap.String("node_id", node.ID),
		zap.Int("tasks", len(node.Tasks)),
		zap.Int("models", len(node.Models)))

	m.notifyWatchers()
}

func (m *MDNSDiscovery) connectToNode(address string) (*grpc.ClientConn, error) {
	baseCtx := m.ctx
	if baseCtx == nil {
		baseCtx = context.Background()
	}
	ctx, cancel := context.WithTimeout(baseCtx, 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, address, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", address, err)
	}
	return conn, nil
}

func (m *MDNSDiscovery) applyCapability(node *NodeInfo, capability *pb.Capability) {
	m.mu.Lock()
	defer m.mu.Unlock()

	node.Capabilities = []*pb.Capability{capability}
	node.Version = m.extractVersionFromCapability(capability)
	node.Runtime = capability.Runtime
	node.Models = m.extractModelsFromCapability(capability)
	node.Tasks = m.extractTasksFromCapability(capability)
	node.Status = NodeStatusActive
	node.InvalidateTaskCache()
}

func (m *MDNSDiscovery) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.cleanupStaleNodes()
		}
	}
}

func (m *MDNSDiscovery) cleanupStaleNodes() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	var staleNodes []string

	for nodeID, node := range m.nodes {
		if now.Sub(node.LastSeen) > m.config.NodeTimeout {
			staleNodes = append(staleNodes, nodeID)
		}
	}

	for _, nodeID := range staleNodes {
		node := m.nodes[nodeID]
		delete(m.nodes, nodeID)
		m.logger.Info("removed stale node",
			zap.String("node_id", nodeID),
			zap.String("name", node.Name),
			zap.Time("last_seen", node.LastSeen))
	}

	if len(staleNodes) > 0 {
		m.notifyWatchers()
	}
}

func (m *MDNSDiscovery) notifyWatchers() {
	m.mu.RLock()
	nodes := make([]*NodeInfo, 0, len(m.nodes))
	for _, node := range m.nodes {
		nodes = append(nodes, node)
	}
	watchers := make([]func([]*NodeInfo), len(m.watchers))
	copy(watchers, m.watchers)
	m.mu.RUnlock()

	for _, watcher := range watchers {
		go watcher(nodes)
	}
}

func (m *MDNSDiscovery) generateNodeID(instance, address string) string {
	return fmt.Sprintf("%s@%s", instance, address)
}

func (m *MDNSDiscovery) extractVersionFromCapability(capability *pb.Capability) string {
	if capability != nil {
		if version, ok := capability.Extra["version"]; ok {
			return version
		}
	}
	return "unknown"
}

func (m *MDNSDiscovery) extractModelsFromCapability(capability *pb.Capability) []*ModelInfo {
	if capability == nil {
		return nil
	}

	var models []*ModelInfo
	for _, modelID := range capability.ModelIds {
		model := &ModelInfo{ID: modelID, Runtime: capability.Runtime}
		models = append(models, model)
	}
	return models
}

func (m *MDNSDiscovery) extractTasksFromCapability(capability *pb.Capability) []*pb.IOTask {
	if capability == nil {
		return nil
	}

	var tasks []*pb.IOTask
	for _, task := range capability.Tasks {
		tasks = append(tasks, task)
	}
	return tasks
}

func (m *MDNSDiscovery) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	activeNodes := 0
	errorNodes := 0
	for _, node := range m.nodes {
		switch node.Status {
		case NodeStatusActive:
			activeNodes++
		case NodeStatusError:
			errorNodes++
		}
	}

	return map[string]interface{}{
		"total_nodes":   len(m.nodes),
		"active_nodes":  activeNodes,
		"error_nodes":   errorNodes,
		"watchers":      len(m.watchers),
		"running":       m.running,
		"service_type":  m.config.ServiceType,
		"domain":        m.config.Domain,
		"scan_interval": m.config.ScanInterval.String(),
		"node_timeout":  m.config.NodeTimeout.String(),
		"max_nodes":     m.config.MaxNodes,
	}
}

// ManualDiscovery is an in-memory backend for tests and explicit registration.
type ManualDiscovery struct {
	nodes    map[string]*NodeInfo
	mu       sync.RWMutex
	watchers []func([]*NodeInfo)
	logger   *zap.Logger
}

func NewManualDiscovery(logger *zap.Logger) *ManualDiscovery {
	return &ManualDiscovery{
		nodes:    make(map[string]*NodeInfo),
		watchers: make([]func([]*NodeInfo), 0),
		logger:   ensureLogger(logger),
	}
}

func (m *ManualDiscovery) Start(ctx context.Context) error {
	_ = ctx
	m.logger.Info("manual discovery started")
	return nil
}

func (m *ManualDiscovery) Stop() error {
	m.logger.Info("manual discovery stopped")
	return nil
}

func (m *ManualDiscovery) GetNodes() []*NodeInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	nodes := make([]*NodeInfo, 0, len(m.nodes))
	for _, node := range m.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

func (m *ManualDiscovery) GetNode(id string) (*NodeInfo, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	node, exists := m.nodes[id]
	return node, exists
}

func (m *ManualDiscovery) Lookup(ctx context.Context, nodeID string) ([]string, error) {
	_ = ctx
	node, exists := m.GetNode(nodeID)
	if !exists || node == nil || strings.TrimSpace(node.Address) == "" {
		return nil, nil
	}
	return []string{node.Address}, nil
}

func (m *ManualDiscovery) Error() error {
	return nil
}

func (m *ManualDiscovery) String() string {
	return "manual"
}

func (m *ManualDiscovery) Cache() map[string]CacheEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cache := make(map[string]CacheEntry, len(m.nodes))
	for id, node := range m.nodes {
		if node == nil {
			continue
		}
		cache[id] = CacheEntry{
			Addresses: UniqueTrimmedStrings([]string{node.Address}),
			When:      node.LastSeen,
			Found:     strings.TrimSpace(node.Address) != "",
		}
	}
	return cache
}

func (m *ManualDiscovery) Watch(callback func([]*NodeInfo)) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.watchers = append(m.watchers, callback)
	return nil
}

func (m *ManualDiscovery) AddNode(node *NodeInfo) {
	if node == nil {
		return
	}

	m.mu.Lock()
	m.nodes[node.ID] = node
	m.mu.Unlock()

	m.notifyWatchers()
	m.logger.Info("manually added node", zap.String("node_id", node.ID), zap.String("name", node.Name))
}

func (m *ManualDiscovery) RemoveNode(nodeID string) {
	m.mu.Lock()
	_, exists := m.nodes[nodeID]
	if exists {
		delete(m.nodes, nodeID)
	}
	m.mu.Unlock()

	if !exists {
		return
	}

	m.notifyWatchers()
	m.logger.Info("manually removed node", zap.String("node_id", nodeID))
}

func (m *ManualDiscovery) notifyWatchers() {
	m.mu.RLock()
	nodes := make([]*NodeInfo, 0, len(m.nodes))
	for _, node := range m.nodes {
		nodes = append(nodes, node)
	}
	watchers := make([]func([]*NodeInfo), len(m.watchers))
	copy(watchers, m.watchers)
	m.mu.RUnlock()

	for _, watcher := range watchers {
		go watcher(nodes)
	}
}
