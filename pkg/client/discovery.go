package client

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"Lumen-SDK/pkg/config"
	"Lumen-SDK/pkg/utils"
	pb "Lumen-SDK/proto"

	"github.com/grandcat/zeroconf"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

// MDNSDiscovery mDNS服务发现实现
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
}

// NewMDNSDiscovery 创建新的mDNS服务发现
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

	ctx, cancel := context.WithCancel(context.Background())

	return &MDNSDiscovery{
		config:   cfg,
		nodes:    make(map[string]*NodeInfo),
		watchers: make([]func([]*NodeInfo), 0),
		ctx:      ctx,
		cancel:   cancel,
		logger:   logger,
	}
}

// Start 启动服务发现
func (m *MDNSDiscovery) Start(ctx context.Context) error {
	if !m.config.Enabled {
		m.logger.Info("mDNS discovery is disabled")
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return utils.DiscoveryFailedError("discovery is already running")
	}

	// 使用utils.SafeExecute安全执行resolver初始化
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
		return utils.Wrap(err, utils.ErrCodeConnectionFailed,
			"failed to initialize mDNS resolver")
	}

	m.running = true

	// 启动定期扫描
	go m.scanLoop(ctx)

	// 启动节点清理
	go m.cleanupLoop(ctx)

	m.logger.Info("mDNS discovery started",
		zap.String("service_type", m.config.ServiceType),
		zap.String("domain", m.config.Domain),
		zap.Duration("scan_interval", m.config.ScanInterval),
	)

	return nil
}

// Stop 停止服务发现
func (m *MDNSDiscovery) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil
	}

	m.logger.Info("stopping mDNS discovery")

	// 使用ErrorAggregator聚合所有停止过程中的错误
	errorAggregator := utils.NewErrorAggregator()

	// 取消context停止所有goroutine
	m.cancel()

	// 安全关闭resolver
	if m.resolver != nil {
		err := utils.SafeExecute(func() error {
			// zeroconf.Resolver没有显式的Close方法
			// 通过将引用置nil来清理
			m.resolver = nil
			return nil
		})
		errorAggregator.Add(err)
	}

	m.running = false

	// 清理节点信息
	nodeCount := len(m.nodes)
	m.nodes = make(map[string]*NodeInfo)
	m.watchers = make([]func([]*NodeInfo), 0)

	if errorAggregator.HasErrors() {
		m.logger.Error("errors occurred during discovery shutdown",
			zap.Int("error_count", len(errorAggregator.GetErrors())),
			zap.Error(errorAggregator.ToError()))
		return utils.ServiceUnavailableError("discovery shutdown failed", errorAggregator.ToError())
	}

	m.logger.Info("mDNS discovery stopped successfully",
		zap.Int("nodes_cleared", nodeCount))

	return nil
}

// GetNodes 获取所有发现的节点
func (m *MDNSDiscovery) GetNodes() []*NodeInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	nodes := make([]*NodeInfo, 0, len(m.nodes))
	for _, node := range m.nodes {
		nodes = append(nodes, node)
	}

	return nodes
}

// GetNode 获取指定节点
func (m *MDNSDiscovery) GetNode(id string) (*NodeInfo, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	node, exists := m.nodes[id]
	return node, exists
}

// Watch 监听节点变化
func (m *MDNSDiscovery) Watch(callback func([]*NodeInfo)) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.watchers = append(m.watchers, callback)

	// 立即通知当前节点列表
	if len(m.nodes) > 0 {
		nodes := make([]*NodeInfo, 0, len(m.nodes))
		for _, node := range m.nodes {
			nodes = append(nodes, node)
		}
		go callback(nodes)
	}

	return nil
}

// scanLoop 定期扫描循环
func (m *MDNSDiscovery) scanLoop(ctx context.Context) {
	ticker := time.NewTicker(m.config.ScanInterval)
	defer ticker.Stop()

	// 立即执行一次扫描
	m.performScanWithRetry(ctx)

	// 创建断路器用于防止连续失败时的资源浪费
	circuitBreaker := utils.NewCircuitBreaker("mdns-scan", 5, 2*time.Minute)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// 使用断路器保护扫描操作
			err := circuitBreaker.Execute(func() error {
				return m.performScanWithRetry(ctx)
			})

			if err != nil {
				m.logger.Error("scan failed due to circuit breaker",
					zap.String("breaker_state", fmt.Sprintf("%v", circuitBreaker.GetState())),
					zap.Error(err))
			}
		}
	}
}

// performScanWithRetry 带重试的扫描执行
func (m *MDNSDiscovery) performScanWithRetry(ctx context.Context) error {
	retryConfig := &utils.RetryConfig{
		Enabled:     true,
		MaxAttempts: 3,
		Backoff:     500 * time.Millisecond,
		MaxBackoff:  10 * time.Second,
		Multiplier:  1.5,
	}

	// 重试回调，记录重试信息
	retryCallback := func(attempt int, err error) {
		m.logger.Warn("scan retry attempt",
			zap.Int("attempt", attempt),
			zap.Error(err))
	}

	return utils.RetryWithCallback(ctx, retryConfig, func(ctx context.Context) error {
		err := m.scanServices()
		if err != nil {
			// 包装为Lumen错误，便于重试策略判断
			if strings.Contains(err.Error(), "timeout") ||
				strings.Contains(err.Error(), "connection") ||
				strings.Contains(err.Error(), "resolver") {
				return utils.NewRetryableError(
					utils.DiscoveryFailedError(err.Error()),
					true,
				)
			}
			// 其他错误类型不重试
			return utils.DiscoveryFailedError(err.Error())
		}
		return nil
	}, retryCallback)
}

// scanServices 扫描服务
func (m *MDNSDiscovery) scanServices() error {
	if !m.running {
		return utils.DiscoveryFailedError("discovery is not running")
	}

	m.logger.Debug("scanning for Lumen services")

	// 设置扫描超时
	scanCtx, cancel := context.WithTimeout(m.ctx, 10*time.Second)
	defer cancel()

	// 执行mDNS扫描
	entries := make(chan *zeroconf.ServiceEntry)
	var scanErr error

	// 执行mDNS扫描
	err := m.resolver.Lookup(scanCtx, "", m.config.ServiceType, m.config.Domain, entries)
	if err != nil {
		return utils.DiscoveryFailedError("mDNS lookup failed",
			map[string]interface{}{
				"error":        err.Error(),
				"service_type": m.config.ServiceType,
				"domain":       m.config.Domain,
			})
	}

	// 处理扫描结果
	for {
		select {
		case entry, ok := <-entries:
			if !ok {
				// entries channel关闭，扫描完成
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

// processServiceEntry 处理服务条目
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

		// 检查是否超过最大节点数
		m.mu.Lock()
		if len(m.nodes) >= m.config.MaxNodes {
			if _, exists := m.nodes[nodeID]; !exists {
				m.mu.Unlock()
				m.logger.Warn("maximum node limit reached, ignoring new node",
					zap.String("node_id", nodeID),
					zap.Int("max_nodes", m.config.MaxNodes),
				)
				continue
			}
		}

		// 更新或创建节点信息
		node, exists := m.nodes[nodeID]
		if !exists {
			node = &NodeInfo{
				ID:       nodeID,
				Name:     entry.Instance,
				Address:  address,
				Status:   NodeStatusStarting,
				Metadata: make(map[string]interface{}),
			}

			// 从TXT记录中提取元数据
			for _, txt := range entry.Text {
				if kv := strings.SplitN(txt, "=", 2); len(kv) == 2 {
					node.Metadata[kv[0]] = kv[1]
				}
			}

			m.nodes[nodeID] = node

			m.logger.Info("discovered new node",
				zap.String("node_id", nodeID),
				zap.String("name", node.Name),
				zap.String("address", node.Address),
			)
		}

		// 更新节点状态
		node.LastSeen = time.Now()
		if node.Status == NodeStatusStarting {
			node.Status = NodeStatusActive
		}

		m.mu.Unlock()

		// 异步获取节点能力
		go m.fetchNodeCapabilities(node)
	}

	// 通知监听器
	m.notifyWatchers()
}

// fetchNodeCapabilities 获取节点能力
func (m *MDNSDiscovery) fetchNodeCapabilities(node *NodeInfo) {
	if !m.running {
		return
	}

	m.logger.Debug("fetching capabilities for node",
		zap.String("node_id", node.ID),
	)

	// 连接到节点并获取能力
	conn, err := m.connectToNode(node.Address)
	if err != nil {

		m.logger.Error("failed to connect to node",
			zap.String("node_id", node.ID),
			zap.Error(err))

		m.mu.Lock()
		node.Status = NodeStatusError
		m.mu.Unlock()
		return
	}
	defer conn.Close()

	client := pb.NewInferenceClient(conn)

	// 设置超时
	ctx, cancel := context.WithTimeout(m.ctx, 10*time.Second)
	defer cancel()

	// 获取能力
	capability, err := client.GetCapabilities(ctx, &emptypb.Empty{})
	if err != nil {

		m.logger.Error("failed to get capabilities from node",
			zap.String("node_id", node.ID),
			zap.Error(err))

		m.mu.Lock()
		node.Status = NodeStatusError
		m.mu.Unlock()
		return
	}

	// 更新节点能力
	m.mu.Lock()
	node.Capabilities = []*pb.Capability{capability}
	node.Version = m.extractVersionFromCapability(capability)
	node.Runtime = capability.Runtime
	node.Models = m.extractModelsFromCapability(capability)
	node.Tasks = m.extractTasksFromCapability(capability)
	node.Status = NodeStatusActive
	m.mu.Unlock()

	m.logger.Info("successfully fetched capabilities for node",
		zap.String("node_id", node.ID),
		zap.Int("tasks", len(node.Tasks)),
		zap.Int("models", len(node.Models)))

	// 通知监听器
	m.notifyWatchers()
}

// connectToNode 连接到节点
func (m *MDNSDiscovery) connectToNode(address string) (*grpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(m.ctx, 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, address,
		grpc.WithInsecure(),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", address, err)
	}

	return conn, nil
}

// cleanupLoop 清理循环
func (m *MDNSDiscovery) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Minute) // 每分钟清理一次
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

// cleanupStaleNodes 清理过期节点
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
			zap.Time("last_seen", node.LastSeen),
		)
	}

	if len(staleNodes) > 0 {
		m.notifyWatchers()
	}
}

// notifyWatchers 通知所有监听器
func (m *MDNSDiscovery) notifyWatchers() {
	m.mu.RLock()
	nodes := make([]*NodeInfo, 0, len(m.nodes))
	for _, node := range m.nodes {
		nodes = append(nodes, node)
	}
	watchers := make([]func([]*NodeInfo), len(m.watchers))
	copy(watchers, m.watchers)
	m.mu.RUnlock()

	// 异步通知所有监听器
	for _, watcher := range watchers {
		go watcher(nodes)
	}
}

// generateNodeID 生成节点ID
func (m *MDNSDiscovery) generateNodeID(instance, address string) string {
	return fmt.Sprintf("%s@%s", instance, address)
}

// extractVersionFromCapability 从能力中提取版本信息
func (m *MDNSDiscovery) extractVersionFromCapability(capability *pb.Capability) string {
	if capability != nil {
		if version, ok := capability.Extra["version"]; ok {
			return version
		}
	}
	return "unknown"
}

// extractModelsFromCapability 从能力中提取模型信息
func (m *MDNSDiscovery) extractModelsFromCapability(capability *pb.Capability) []*ModelInfo {
	if capability == nil {
		return nil
	}

	var models []*ModelInfo
	for _, modelID := range capability.ModelIds {
		model := &ModelInfo{
			ID:      modelID,
			Runtime: capability.Runtime,
		}
		models = append(models, model)
	}

	return models
}

// extractTasksFromCapability 从能力中提取任务信息
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

// GetStats 获取发现服务统计信息
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

// ManualDiscovery 手动发现实现（用于测试）
type ManualDiscovery struct {
	nodes    map[string]*NodeInfo
	mu       sync.RWMutex
	watchers []func([]*NodeInfo)
	logger   *zap.Logger
}

// NewManualDiscovery 创建手动发现实例
func NewManualDiscovery(logger *zap.Logger) *ManualDiscovery {
	return &ManualDiscovery{
		nodes:    make(map[string]*NodeInfo),
		watchers: make([]func([]*NodeInfo), 0),
		logger:   logger,
	}
}

// Start 启动手动发现
func (m *ManualDiscovery) Start(ctx context.Context) error {
	m.logger.Info("manual discovery started")
	return nil
}

// Stop 停止手动发现
func (m *ManualDiscovery) Stop() error {
	m.logger.Info("manual discovery stopped")
	return nil
}

// GetNodes 获取所有节点
func (m *ManualDiscovery) GetNodes() []*NodeInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	nodes := make([]*NodeInfo, 0, len(m.nodes))
	for _, node := range m.nodes {
		nodes = append(nodes, node)
	}

	return nodes
}

// GetNode 获取指定节点
func (m *ManualDiscovery) GetNode(id string) (*NodeInfo, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	node, exists := m.nodes[id]
	return node, exists
}

// Watch 监听节点变化
func (m *ManualDiscovery) Watch(callback func([]*NodeInfo)) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.watchers = append(m.watchers, callback)
	return nil
}

// AddNode 手动添加节点
func (m *ManualDiscovery) AddNode(node *NodeInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if node == nil {
		return
	}

	m.nodes[node.ID] = node
	m.notifyWatchers()

	m.logger.Info("manually added node",
		zap.String("node_id", node.ID),
		zap.String("name", node.Name))
}

// RemoveNode 手动移除节点
func (m *ManualDiscovery) RemoveNode(nodeID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.nodes[nodeID]; exists {
		delete(m.nodes, nodeID)
		m.notifyWatchers()

		m.logger.Info("manually removed node", zap.String("node_id", nodeID))
	}
}

// notifyWatchers 通知监听器
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
