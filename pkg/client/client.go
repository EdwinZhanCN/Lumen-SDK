package client

import (
	"context"
	"fmt"
	"sync"
	"time"

	"Lumen-SDK/pkg/config"
	pb "Lumen-SDK/proto"

	"go.uber.org/zap"
)

// ClientMetrics 客户端指标
type ClientMetrics struct {
	TotalRequests      int64         `json:"total_requests"`
	SuccessfulRequests int64         `json:"successful_requests"`
	FailedRequests     int64         `json:"failed_requests"`
	ErrorRate          float64       `json:"error_rate"`
	AverageLatency     time.Duration `json:"average_latency"`
	ThroughputQPS      float64       `json:"throughput_qps"`
	ActiveNodes        int           `json:"active_nodes"`
	TotalNodes         int           `json:"total_nodes"`
	LastUpdated        time.Time     `json:"last_updated"`
}

// LumenClient Lumen客户端实现
type LumenClient struct {
	config    *config.Config
	discovery *MDNSDiscovery
	pool      *GRPCConnectionPool
	balancer  LoadBalancer
	logger    *zap.Logger
	running   bool
	metrics   *ClientMetrics
	mu        sync.RWMutex
}

// NewLumenClient 创建新的Lumen客户端
func NewLumenClient(cfg *config.Config, logger *zap.Logger) (*LumenClient, error) {
	return NewLumenClientWithBalancer(cfg, logger, RoundRobin)
}

// NewLumenClientWithBalancer 创建指定负载均衡策略的Lumen客户端
func NewLumenClientWithBalancer(cfg *config.Config, logger *zap.Logger, balancerType LoadBalancerType) (*LumenClient, error) {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	// 更新配置中的策略
	cfg.LoadBalancer.Strategy = string(balancerType)

	// 创建负载均衡器
	balancer := CreateLoadBalancer(balancerType, &cfg.LoadBalancer, logger)

	client := &LumenClient{
		config:   cfg,
		balancer: balancer,
		logger:   logger,
		running:  false,
		metrics: &ClientMetrics{
			LastUpdated: time.Now(),
		},
	}

	// 初始化组件
	if err := client.initializeComponents(); err != nil {
		return nil, fmt.Errorf("failed to initialize client components: %w", err)
	}

	return client, nil
}

// initializeComponents 初始化客户端组件
func (c *LumenClient) initializeComponents() error {
	// 初始化服务发现
	c.discovery = NewMDNSDiscovery(&c.config.Discovery, c.logger)

	// 初始化连接池，使用默认配置
	c.pool = NewGRPCConnectionPool(nil, c.logger)

	return nil
}

// Start 启动客户端
func (c *LumenClient) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		return fmt.Errorf("client is already running")
	}

	// 启动服务发现
	if err := c.discovery.Start(ctx); err != nil {
		return fmt.Errorf("failed to start service discovery: %w", err)
	}

	// 监听节点变化
	if err := c.discovery.Watch(c.onNodesChanged); err != nil {
		c.logger.Error("failed to watch nodes", zap.Error(err))
	}

	// 启动连接池维护循环
	go c.pool.StartMaintenanceLoop(ctx)

	// 启动指标收集循环
	go c.metricsCollectionLoop(ctx)

	c.running = true

	c.logger.Info("Lumen client started successfully")
	return nil
}

// Stop 停止客户端
func (c *LumenClient) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running {
		return nil
	}

	// 停止服务发现
	c.discovery.Stop()

	// 关闭负载均衡器
	if err := c.balancer.Close(); err != nil {
		c.logger.Error("failed to close load balancer", zap.Error(err))
	}

	// 关闭连接池
	if err := c.pool.Close(); err != nil {
		c.logger.Error("failed to close connection pool", zap.Error(err))
	}

	c.running = false

	c.logger.Info("Lumen client stopped")
	return nil
}

// Infer 执行非流式推理请求
// 对于简单的单次推理请求，此方法会自动处理双向流的复杂性
// 发送一个请求并等待一个响应，然后关闭流
func (c *LumenClient) Infer(ctx context.Context, req *pb.InferRequest) (*pb.InferResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	startTime := time.Now()
	c.incrementTotalRequests()

	// 使用负载均衡器选择节点
	node, err := c.balancer.SelectNode(ctx, req.Task)
	if err != nil {
		c.incrementFailedRequests()
		return nil, fmt.Errorf("failed to select node: %w", err)
	}

	// 增加节点连接计数
	node.IncrementConnections()
	defer node.DecrementConnections()

	// 获取连接
	conn, err := c.pool.GetConnection(node.ID)
	if err != nil {
		c.incrementFailedRequests()
		return nil, fmt.Errorf("failed to get connection for node %s: %w", node.ID, err)
	}

	// 执行推理
	resp, err := conn.Infer(ctx, req)
	if err != nil {
		c.incrementFailedRequests()
		return nil, fmt.Errorf("inference failed: %w", err)
	}

	// 返回连接
	c.pool.ReturnConnection(node.ID, conn)

	// 更新统计
	c.incrementSuccessfulRequests()
	c.updateLatency(time.Since(startTime))

	return resp, nil
}

// InferStream 执行流式推理请求
// 返回一个通道，可以接收多个响应（部分结果和最终结果）
// 适用于需要增量结果的场景，如生成式AI、流式音频处理等
func (c *LumenClient) InferStream(ctx context.Context, req *pb.InferRequest) (<-chan *pb.InferResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	// 选择节点
	node, err := c.balancer.SelectNode(ctx, req.Task)
	if err != nil {
		return nil, fmt.Errorf("failed to select node: %w", err)
	}

	// 获取连接
	conn, err := c.pool.GetConnection(node.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection for node %s: %w", node.ID, err)
	}

	// 执行流式推理
	respChan, err := c.InferStream(ctx, req)
	if err != nil {
		c.incrementFailedRequests()
		c.pool.ReturnConnection(node.ID, conn)
		return nil, fmt.Errorf("stream inference failed: %w", err)
	}

	// 创建包装通道，用于连接管理
	wrappedChan := make(chan *pb.InferResponse, 100)

	// 启动流处理和连接管理
	go func() {
		defer close(wrappedChan)
		defer c.pool.ReturnConnection(node.ID, conn)

		// 转发响应
		for resp := range respChan {
			wrappedChan <- resp
			if resp.IsFinal {
				break
			}
		}
	}()

	return wrappedChan, nil
}

// GetBalancerStats 获取负载均衡器统计信息
func (c *LumenClient) GetBalancerStats() LoadBalancerStats {
	return c.balancer.GetStats()
}

// GetMetrics 获取客户端指标
func (c *LumenClient) GetMetrics() *ClientMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	metrics := *c.metrics
	return &metrics
}

// GetCapabilities 获取节点能力
func (c *LumenClient) GetCapabilities(ctx context.Context, nodeID string) ([]*pb.Capability, error) {
	node, exists := c.discovery.GetNode(nodeID)
	if !exists {
		return nil, fmt.Errorf("node not found: %s", nodeID)
	}

	return node.Capabilities, nil
}

// GetNodes 获取所有节点
func (c *LumenClient) GetNodes() []*NodeInfo {
	return c.discovery.GetNodes()
}

// GetNode 获取指定节点
func (c *LumenClient) GetNode(id string) (*NodeInfo, bool) {
	return c.discovery.GetNode(id)
}

// WatchNodes 监听节点变化
func (c *LumenClient) WatchNodes(callback func([]*NodeInfo)) error {
	return c.discovery.Watch(callback)
}

// Close 关闭客户端
func (c *LumenClient) Close() error {
	return c.Stop()
}

// 节点变化回调
// onNodesChanged 节点变化回调
func (c *LumenClient) onNodesChanged(nodes []*NodeInfo) {
	c.logger.Debug("nodes changed", zap.Int("count", len(nodes)))

	// 更新负载均衡器的节点列表
	c.balancer.UpdateNodes(nodes)

	// 更新连接池中的节点
	for _, node := range nodes {
		if node.IsActive() {
			// 节点变为活跃，可以预建连接
			c.pool.EnsureConnection(node.ID, node.Address)
		} else {
			// 节点变为不活跃，清理连接
			c.pool.RemoveConnection(node.ID)
		}
	}
}

// 统计方法
func (c *LumenClient) incrementTotalRequests() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metrics.TotalRequests++
}

func (c *LumenClient) incrementSuccessfulRequests() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metrics.SuccessfulRequests++
}

func (c *LumenClient) incrementFailedRequests() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metrics.FailedRequests++
}

func (c *LumenClient) updateLatency(duration time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.metrics.AverageLatency == 0 {
		c.metrics.AverageLatency = duration
	} else {
		// 指数移动平均
		alpha := 0.1
		c.metrics.AverageLatency = time.Duration(
			float64(c.metrics.AverageLatency)*(1-alpha) + float64(duration)*alpha)
	}
}

func (c *LumenClient) metricsCollectionLoop(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.mu.Lock()
			c.metrics.LastUpdated = time.Now()
			c.metrics.ActiveNodes = len(c.discovery.GetNodes())
			c.metrics.TotalNodes = c.metrics.ActiveNodes

			// 计算错误率
			if c.metrics.TotalRequests > 0 {
				c.metrics.ErrorRate = float64(c.metrics.FailedRequests) / float64(c.metrics.TotalRequests)
			}

			// 计算QPS
			c.metrics.ThroughputQPS = float64(c.metrics.SuccessfulRequests) / 10.0 // 10秒窗口

			c.mu.Unlock()
		}
	}
}
