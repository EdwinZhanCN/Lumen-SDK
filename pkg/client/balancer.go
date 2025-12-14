package client

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/edwinzhancn/lumen-sdk/pkg/config"

	"go.uber.org/zap"
)

// LoadBalancer defines the interface for distributing inference requests across ML nodes.
//
// Load balancers implement strategies for node selection to optimize:
//   - Resource utilization across nodes
//   - Response time and latency
//   - Task-specific routing based on node capabilities
//   - Fault tolerance and failover
//
// Built-in strategies include round-robin, random, weighted, least-connections,
// and task-aware variants that consider node capabilities.
//
// Role in project: Critical component for distributing workload across the distributed
// ML cluster. Proper load balancing ensures optimal performance and resource utilization.
//
// Example:
//
//	// Create load balancer with strategy
//	balancer := client.CreateLoadBalancer(
//	    client.RoundRobin,
//	    &cfg.LoadBalancer,
//	    logger,
//	)
//
//	// Select node for inference
//	node, err := balancer.SelectNode(ctx, "text_embedding")
type LoadBalancer interface {
	// SelectNode 选择一个合适的节点来处理指定任务
	SelectNode(ctx context.Context, task string) (*NodeInfo, error)

	// UpdateNodes 更新节点列表
	UpdateNodes(nodes []*NodeInfo)

	// GetStats 获取负载均衡器统计信息
	GetStats() LoadBalancerStats

	// Close 关闭负载均衡器
	Close() error
}

// LoadBalancerStats 负载均衡器统计信息
type LoadBalancerStats struct {
	TotalNodes      int64     `json:"total_nodes"`
	ActiveNodes     int64     `json:"active_nodes"`
	SelectionsCount int64     `json:"selections_count"`
	LastSelection   time.Time `json:"last_selection"`
	Strategy        string    `json:"strategy"`
}

// LoadBalancingStrategy 负载均衡策略接口
type LoadBalancingStrategy interface {
	// Select 从节点列表中选择一个节点
	Select(ctx context.Context, nodes []*NodeInfo, task string) (*NodeInfo, error)

	// Name 策略名称
	Name() string
}

// HealthCheckFunc is a function type for performing actual health checks on nodes.
// It should return true if the node is healthy, false otherwise.
// When health check succeeds, implementations should update node.LastSeen.
type HealthCheckFunc func(node *NodeInfo) bool

// SimpleLoadBalancer 简单的负载均衡器实现
type SimpleLoadBalancer struct {
	strategy        LoadBalancingStrategy
	config          *config.LoadBalancerConfig
	cache           *NodeCache
	healthChecker   *HealthChecker
	nodes           []*NodeInfo
	mu              sync.RWMutex
	logger          *zap.Logger
	stats           LoadBalancerStats
	closed          bool
	healthCheckFunc HealthCheckFunc // Optional external health check function
}

// NewSimpleLoadBalancer 创建带配置的负载均衡器
func NewSimpleLoadBalancer(strategy LoadBalancingStrategy, config *config.LoadBalancerConfig, logger *zap.Logger) *SimpleLoadBalancer {
	lb := &SimpleLoadBalancer{
		strategy: strategy,
		nodes:    make([]*NodeInfo, 0),
		logger:   logger,
		stats: LoadBalancerStats{
			Strategy: strategy.Name(),
		},
	}

	// 如果提供了配置，应用配置
	if config != nil {
		lb.config = config

		// 初始化缓存（如果启用）
		if config.CacheEnabled {
			lb.cache = NewNodeCache(config.CacheTTL)
		}

		// 初始化健康检查（如果启用）
		if config.HealthCheck {
			lb.healthChecker = NewHealthChecker(config.CheckInterval, logger)
		}
	}

	return lb
}

// SelectNode 选择节点
func (lb *SimpleLoadBalancer) SelectNode(ctx context.Context, task string) (*NodeInfo, error) {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	if lb.closed {
		return nil, fmt.Errorf("load balancer is closed")
	}

	// 1. 检查缓存 (如果启用)
	if lb.config != nil && lb.config.CacheEnabled && lb.cache != nil {
		if cached := lb.cache.Get(task); cached != nil {
			lb.logger.Debug("node selected from cache",
				zap.String("node_id", cached.ID),
				zap.String("task", task))

			lb.stats.SelectionsCount++
			lb.stats.LastSelection = time.Now()
			return cached, nil
		}
	}

	if len(lb.nodes) == 0 {
		return nil, fmt.Errorf("no nodes available")
	}

	// 2. 过滤可用节点 (包含健康检查)
	availableNodes := lb.filterAvailableNodes(lb.nodes, task)
	if len(availableNodes) == 0 {
		return nil, fmt.Errorf("no suitable nodes available for task: %s", task)
	}

	// 3. 使用策略选择节点
	selectedNode, err := lb.strategy.Select(ctx, availableNodes, task)
	if err != nil {
		return nil, fmt.Errorf("strategy selection failed: %w", err)
	}

	// 4. 缓存结果 (如果启用)
	if lb.config != nil && lb.config.CacheEnabled && lb.cache != nil {
		lb.cache.Set(task, selectedNode, lb.config.CacheTTL)
	}

	// 5. 更新统计
	lb.stats.SelectionsCount++
	lb.stats.LastSelection = time.Now()

	lb.logger.Debug("node selected",
		zap.String("node_id", selectedNode.ID),
		zap.String("task", task),
		zap.String("strategy", lb.strategy.Name()))

	return selectedNode, nil
}

// UpdateNodes 更新节点列表
func (lb *SimpleLoadBalancer) UpdateNodes(nodes []*NodeInfo) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	lb.nodes = make([]*NodeInfo, len(nodes))
	copy(lb.nodes, nodes)

	lb.stats.TotalNodes = int64(len(nodes))

	// 统计活跃节点
	activeCount := 0
	for _, node := range nodes {
		if node.IsActive() {
			activeCount++
		}
	}
	lb.stats.ActiveNodes = int64(activeCount)

	// 启动健康检查（如果启用且节点列表不为空）
	if lb.config != nil && lb.config.HealthCheck && lb.healthChecker != nil && len(nodes) > 0 {
		lb.healthChecker.setNodes(nodes)
		go lb.healthChecker.Start(context.Background(), lb.createHealthCheckFunction())
	}

	lb.logger.Debug("nodes updated",
		zap.Int("total", len(nodes)),
		zap.Int("active", activeCount),
		zap.String("strategy", lb.strategy.Name()))
}

// SetHealthCheckFunc sets an external health check function that performs
// actual gRPC health checks. This should be called by LumenClient after
// creating the load balancer.
func (lb *SimpleLoadBalancer) SetHealthCheckFunc(fn HealthCheckFunc) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.healthCheckFunc = fn
}

// createHealthCheckFunction 创建健康检查函数
func (lb *SimpleLoadBalancer) createHealthCheckFunction() func(*NodeInfo) bool {
	return func(node *NodeInfo) bool {
		// If an external health check function is set, use it
		lb.mu.RLock()
		externalCheck := lb.healthCheckFunc
		lb.mu.RUnlock()

		if externalCheck != nil {
			healthy := externalCheck(node)
			if healthy {
				// Update LastSeen on successful health check
				node.LastSeen = time.Now()
				if node.Status == NodeStatusError {
					node.Status = NodeStatusActive
				}
			}
			return healthy
		}

		// Fallback: simple status-based check (no LastSeen timeout)
		// When no external health check is configured, trust the node status
		if node.Status == NodeStatusUnknown || node.Status == NodeStatusError {
			return false
		}

		return true
	}
}

// GetStats 获取统计信息
func (lb *SimpleLoadBalancer) GetStats() LoadBalancerStats {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	stats := lb.stats
	stats.TotalNodes = int64(len(lb.nodes))

	// 实时统计活跃节点
	activeCount := 0
	for _, node := range lb.nodes {
		if node.IsActive() {
			activeCount++
		}
	}
	stats.ActiveNodes = int64(activeCount)

	return stats
}

// Close 关闭负载均衡器
func (lb *SimpleLoadBalancer) Close() error {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	lb.closed = true

	// 关闭缓存
	if lb.cache != nil {
		lb.cache.Close()
	}

	// 停止健康检查
	if lb.healthChecker != nil {
		lb.healthChecker.Stop()
	}

	lb.logger.Info("load balancer closed", zap.String("strategy", lb.strategy.Name()))
	return nil
}

// filterAvailableNodes 过滤可用节点
func (lb *SimpleLoadBalancer) filterAvailableNodes(nodes []*NodeInfo, task string) []*NodeInfo {
	available := make([]*NodeInfo, 0)

	for _, node := range nodes {
		// 检查节点是否活跃
		if !node.IsActive() {
			continue
		}

		// 如果启用了健康检查，只选择健康节点
		if lb.config != nil && lb.config.HealthCheck {
			if node.Status == NodeStatusError {
				continue
			}
		}

		// 检查节点是否支持指定任务
		if !node.SupportsTask(task) {
			continue
		}

		available = append(available, node)
	}

	return available
}

// ============== 负载均衡策略实现 ==============

// RoundRobinStrategy 轮询策略
type RoundRobinStrategy struct {
	current int64
	mu      sync.Mutex
}

// NewRoundRobinStrategy 创建轮询策略
func NewRoundRobinStrategy() *RoundRobinStrategy {
	return &RoundRobinStrategy{}
}

func (s *RoundRobinStrategy) Select(ctx context.Context, nodes []*NodeInfo, task string) (*NodeInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(nodes) == 0 {
		return nil, fmt.Errorf("no nodes available")
	}

	index := int(s.current) % len(nodes)
	s.current++

	return nodes[index], nil
}

func (s *RoundRobinStrategy) Name() string {
	return "round_robin"
}

// RandomStrategy 随机策略
type RandomStrategy struct {
	rand *rand.Rand
	mu   sync.Mutex
}

// NewRandomStrategy 创建随机策略
func NewRandomStrategy() *RandomStrategy {
	return &RandomStrategy{
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (s *RandomStrategy) Select(ctx context.Context, nodes []*NodeInfo, task string) (*NodeInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(nodes) == 0 {
		return nil, fmt.Errorf("no nodes available")
	}

	index := s.rand.Intn(len(nodes))
	return nodes[index], nil
}

func (s *RandomStrategy) Name() string {
	return "random"
}

// WeightedStrategy 加权策略
type WeightedStrategy struct {
	rand *rand.Rand
	mu   sync.Mutex
}

// NewWeightedStrategy 创建加权策略
func NewWeightedStrategy() *WeightedStrategy {
	return &WeightedStrategy{
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (s *WeightedStrategy) Select(ctx context.Context, nodes []*NodeInfo, task string) (*NodeInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(nodes) == 0 {
		return nil, fmt.Errorf("no nodes available")
	}

	// 计算总权重
	totalWeight := int64(0)
	for _, node := range nodes {
		totalWeight += node.Weight
	}

	if totalWeight <= 0 {
		// 如果没有权重配置，回退到随机选择
		index := s.rand.Intn(len(nodes))
		return nodes[index], nil
	}

	// 随机选择
	targetWeight := s.rand.Int63n(totalWeight)
	currentWeight := int64(0)

	for _, node := range nodes {
		currentWeight += node.Weight
		if currentWeight > targetWeight {
			return node, nil
		}
	}

	// 理论上不应该到达这里，但为了安全返回最后一个节点
	return nodes[len(nodes)-1], nil
}

func (s *WeightedStrategy) Name() string {
	return "weighted"
}

// ============== 缓存和健康检查实现 ==============

// NodeCache 节点缓存
type NodeCache struct {
	cache  map[string]*NodeInfo
	ttl    time.Duration
	mu     sync.RWMutex
	closed bool
}

// NewNodeCache 创建节点缓存
func NewNodeCache(ttl time.Duration) *NodeCache {
	cache := &NodeCache{
		cache: make(map[string]*NodeInfo),
		ttl:   ttl,
	}

	// 启动过期清理
	go cache.startCleanup()

	return cache
}

// Get 从缓存获取节点
func (c *NodeCache) Get(task string) *NodeInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.cache[task]
}

// Set 设置缓存
func (c *NodeCache) Set(task string, node *NodeInfo, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache[task] = node
}

// Invalidate 使缓存失效
func (c *NodeCache) Invalidate(task string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.cache, task)
}

// Clear 清空缓存
func (c *NodeCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = make(map[string]*NodeInfo)
}

// Close 关闭缓存
func (c *NodeCache) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.closed = true
	c.cache = nil
}

// startCleanup 启动过期清理
func (c *NodeCache) startCleanup() {
	ticker := time.NewTicker(c.ttl / 2)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.mu.Lock()
			if c.closed {
				c.mu.Unlock()
				return
			}

			// 简单的过期策略：定时清空
			c.cache = make(map[string]*NodeInfo)
			c.mu.Unlock()
		case <-time.After(c.ttl):
			// 超时保护
			return
		}
	}
}

// HealthChecker 健康检查器
type HealthChecker struct {
	interval time.Duration
	logger   *zap.Logger
	stopCh   chan struct{}
	nodes    []*NodeInfo
	mu       sync.RWMutex
}

// NewHealthChecker 创建健康检查器
func NewHealthChecker(interval time.Duration, logger *zap.Logger) *HealthChecker {
	return &HealthChecker{
		interval: interval,
		logger:   logger,
		stopCh:   make(chan struct{}),
	}
}

// Start 启动健康检查
func (hc *HealthChecker) Start(ctx context.Context, checkFunc func(*NodeInfo) bool) {
	ticker := time.NewTicker(hc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-hc.stopCh:
			return
		case <-ticker.C:
			hc.checkNodesFunc(checkFunc)
		}
	}
}

// Stop 停止健康检查
func (hc *HealthChecker) Stop() {
	close(hc.stopCh)
}

// checkNodes 检查节点健康状态
func (hc *HealthChecker) checkNodes(nodes []*NodeInfo) {
	for _, node := range nodes {
		go hc.checkNodeFunc(node, hc.createDefaultCheck())
	}
}

// checkNodesFunc 使用自定义检查函数检查节点
func (hc *HealthChecker) checkNodesFunc(checkFunc func(*NodeInfo) bool) {
	for _, node := range hc.nodes {
		go hc.checkNodeFunc(node, checkFunc)
	}
}

// checkNodeFunc 使用自定义函数检查单个节点
func (hc *HealthChecker) checkNodeFunc(node *NodeInfo, checkFunc func(*NodeInfo) bool) {
	if checkFunc(node) {
		// 健康检查通过
		if node.Status == NodeStatusError {
			node.Status = NodeStatusActive
		}

		hc.logger.Debug("node health check passed",
			zap.String("node_id", node.ID))
	} else {
		hc.logger.Warn("node health check failed",
			zap.String("node_id", node.ID))

		// 更新节点状态
		node.Status = NodeStatusError
	}
}

// createDefaultCheck 创建默认的健康检查函数
func (hc *HealthChecker) createDefaultCheck() func(*NodeInfo) bool {
	return func(node *NodeInfo) bool {
		// Simple status-based check without LastSeen timeout
		// The actual health check should be done via gRPC and update LastSeen
		if node.Status == NodeStatusUnknown || node.Status == NodeStatusError {
			return false
		}

		return true
	}
}

// setNodes 设置要检查的节点列表
func (hc *HealthChecker) setNodes(nodes []*NodeInfo) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.nodes = nodes
}

// LeastConnectionsStrategy 最少连接策略
type LeastConnectionsStrategy struct{}

// NewLeastConnectionsStrategy 创建最少连接策略
func NewLeastConnectionsStrategy() *LeastConnectionsStrategy {
	return &LeastConnectionsStrategy{}
}

func (s *LeastConnectionsStrategy) Select(ctx context.Context, nodes []*NodeInfo, task string) (*NodeInfo, error) {
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no nodes available")
	}

	// 找到连接数最少的节点
	var selected *NodeInfo
	minConnections := int64(-1)

	for _, node := range nodes {
		connections := node.GetConnections()
		if minConnections == -1 || connections < minConnections {
			minConnections = connections
			selected = node
		}
	}

	return selected, nil
}

func (s *LeastConnectionsStrategy) Name() string {
	return "least_connections"
}

// TaskAwareStrategy 任务感知策略
type TaskAwareStrategy struct {
	strategy LoadBalancingStrategy // 底层策略
}

// NewTaskAwareStrategy 创建任务感知策略
func NewTaskAwareStrategy(baseStrategy LoadBalancingStrategy) *TaskAwareStrategy {
	return &TaskAwareStrategy{
		strategy: baseStrategy,
	}
}

func (s *TaskAwareStrategy) Select(ctx context.Context, nodes []*NodeInfo, task string) (*NodeInfo, error) {
	// 根据任务类型对节点进行排序和过滤
	sortedNodes := s.rankNodesByTask(nodes, task)

	return s.strategy.Select(ctx, sortedNodes, task)
}

func (s *TaskAwareStrategy) Name() string {
	return fmt.Sprintf("task_aware_%s", s.strategy.Name())
}

// rankNodesByTask 根据任务类型对节点排序
func (s *TaskAwareStrategy) rankNodesByTask(nodes []*NodeInfo, task string) []*NodeInfo {
	// 复制节点列表
	sorted := make([]*NodeInfo, len(nodes))
	copy(sorted, nodes)

	// 根据节点对任务的适合度排序
	sort.Slice(sorted, func(i, j int) bool {
		scoreI := s.calculateTaskScore(sorted[i], task)
		scoreJ := s.calculateTaskScore(sorted[j], task)
		return scoreI > scoreJ
	})

	return sorted
}

// calculateTaskScore 计算节点对任务的适合度分数
func (s *TaskAwareStrategy) calculateTaskScore(node *NodeInfo, task string) float64 {
	score := 0.0

	// 检查是否支持任务
	if !node.SupportsTask(task) {
		return -1.0
	}

	// 基础分数
	score += 10.0

	// 根据节点能力加分
	for _, capability := range node.Capabilities {
		for _, supportedTask := range capability.Tasks {
			if supportedTask.Name == task {
				score += 20.0 // 完全匹配

				// 根据硬件能力加分
				switch capability.Runtime {
				case "cuda", "tensorrt":
					score += 10.0
				case "coreml", "ane":
					score += 8.0
				default:
					score += 2.0
				}

				// 根据精度支持加分
				for _, precision := range capability.Precisions {
					if precision == "int8" {
						score += 2.0
					} else if precision == "fp16" {
						score += 1.5
					}
				}

				// 根据并发能力加分
				score += float64(capability.MaxConcurrency) * 0.1
			}
		}
	}

	// 根据当前负载扣分
	if node.Load != nil {
		score -= node.Load.CPU * 5.0
		score -= node.Load.Memory * 5.0
	}

	// 根据错误率扣分
	if node.Stats != nil && node.Stats.TotalRequests > 0 {
		errorRate := float64(node.Stats.FailedRequests) / float64(node.Stats.TotalRequests)
		score -= errorRate * 20.0
	}

	return score
}

// ============== 工厂函数 ==============

// LoadBalancerType represents the load balancing strategy to use.
//
// Each type implements different selection logic:
//   - RoundRobin: Cycles through nodes sequentially
//   - Random: Selects nodes randomly
//   - Weighted: Considers node weight/capacity
//   - LeastConnections: Selects node with fewest active connections
//   - TaskAwareRoundRobin: Round-robin with task capability matching
//   - TaskAwareRandom: Random selection with task capability matching
type LoadBalancerType string

const (
	RoundRobin          LoadBalancerType = "round_robin"            // Sequential node selection
	Random              LoadBalancerType = "random"                 // Random node selection
	Weighted            LoadBalancerType = "weighted"               // Weight-based selection
	LeastConnections    LoadBalancerType = "least_connections"      // Connection-based selection
	TaskAwareRoundRobin LoadBalancerType = "task_aware_round_robin" // Task-aware round-robin
	TaskAwareRandom     LoadBalancerType = "task_aware_random"      // Task-aware random
)

// CreateLoadBalancer creates a configured load balancer with the specified strategy.
//
// This factory function instantiates the appropriate load balancing strategy and
// wraps it in a SimpleLoadBalancer with optional caching and health checking.
// If an unknown type is provided, it falls back to round-robin with a warning.
//
// Parameters:
//   - balancerType: The load balancing strategy to use
//   - config: Load balancer configuration (caching, health checks, etc.)
//   - logger: Logger for load balancer operations
//
// Returns:
//   - LoadBalancer: Configured load balancer ready for node selection
//
// Role in project: Factory function that creates load balancers with proper configuration.
// This is the recommended way to instantiate load balancers in the SDK.
//
// Example:
//
//	// Create round-robin balancer
//	balancer := client.CreateLoadBalancer(
//	    client.RoundRobin,
//	    &cfg.LoadBalancer,
//	    logger,
//	)
//
//	// Create task-aware balancer with caching
//	cfg.LoadBalancer.CacheEnabled = true
//	cfg.LoadBalancer.CacheTTL = 5 * time.Minute
//	balancer := client.CreateLoadBalancer(
//	    client.TaskAwareRoundRobin,
//	    &cfg.LoadBalancer,
//	    logger,
//	)
func CreateLoadBalancer(balancerType LoadBalancerType, config *config.LoadBalancerConfig, logger *zap.Logger) LoadBalancer {
	var strategy LoadBalancingStrategy

	switch balancerType {
	case RoundRobin:
		strategy = NewRoundRobinStrategy()
	case Random:
		strategy = NewRandomStrategy()
	case Weighted:
		strategy = NewWeightedStrategy()
	case LeastConnections:
		strategy = NewLeastConnectionsStrategy()
	case TaskAwareRoundRobin:
		strategy = NewTaskAwareStrategy(NewRoundRobinStrategy())
	case TaskAwareRandom:
		strategy = NewTaskAwareStrategy(NewRandomStrategy())
	default:
		// 默认使用轮询
		strategy = NewRoundRobinStrategy()
		logger.Warn("unknown load balancer type, using round_robin",
			zap.String("type", string(balancerType)))
	}

	return NewSimpleLoadBalancer(strategy, config, logger)
}
