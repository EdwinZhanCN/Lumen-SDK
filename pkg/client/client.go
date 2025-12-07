package client

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/edwinzhancn/lumen-sdk/pkg/config"
	pb "github.com/edwinzhancn/lumen-sdk/proto"

	"go.uber.org/zap"
)

// ClientMetrics tracks performance and operational metrics for the Lumen client.
//
// This structure provides real-time insights into the client's behavior and health,
// including request statistics, error rates, latency measurements, and node availability.
// These metrics are useful for monitoring, debugging, and optimizing client operations.
//
// Role in project: Provides observability and monitoring data for the client layer,
// enabling administrators and developers to track system performance and diagnose issues.
//
// Example:
//
//	client, _ := client.NewLumenClient(cfg, logger)
//	metrics := client.GetMetrics()
//	fmt.Printf("Total requests: %d\n", metrics.TotalRequests)
//	fmt.Printf("Error rate: %.2f%%\n", metrics.ErrorRate*100)
//	fmt.Printf("Average latency: %v\n", metrics.AverageLatency)
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

// LumenClient is the main client for interacting with the Lumen distributed AI service platform.
//
// LumenClient handles all aspects of communication with Lumen ML nodes, including:
//   - Service discovery and node management via mDNS
//   - Load balancing across multiple nodes using configurable strategies
//   - Connection pooling for efficient resource utilization
//   - Automatic request chunking for large payloads
//   - Metrics collection and monitoring
//
// Role in project: Acts as the primary entry point for applications to perform ML inference
// operations. It abstracts away the complexity of distributed node management, load balancing,
// and network communication, providing a simple API for users.
//
// Example usage:
//
//	// Create configuration
//	cfg := config.DefaultConfig()
//	logger, _ := zap.NewProduction()
//
//	// Create client
//	client, err := client.NewLumenClient(cfg, logger)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Start the client
//	ctx := context.Background()
//	if err := client.Start(ctx); err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Close()
//
//	// Perform inference
//	textData := []byte("Hello, world!")
//	embeddingReq, _ := types.NewEmbeddingRequest(textData)
//	inferReq := types.NewInferRequest("text_embedding").
//	    ForEmbedding(embeddingReq, "text_embedding").
//	    Build()
//	result, err := client.Infer(ctx, inferReq)
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

// NewLumenClient creates a new LumenClient instance with default round-robin load balancing.
//
// This function initializes all client components including service discovery, connection pooling,
// and load balancing. If cfg is nil, default configuration is used. If logger is nil, you should
// provide a valid logger for proper operation.
//
// Parameters:
//   - cfg: Configuration for the client. Pass nil to use defaults.
//   - logger: Logger instance for client operations. Should not be nil in production.
//
// Returns:
//   - *LumenClient: Initialized client ready to be started
//   - error: Initialization error if any component fails to set up
//
// Role in project: Factory function that creates the main client object. This is the
// primary way applications instantiate the Lumen client.
//
// Example:
//
//	cfg := config.DefaultConfig()
//	logger, _ := zap.NewProduction()
//	client, err := client.NewLumenClient(cfg, logger)
//	if err != nil {
//	    log.Fatalf("Failed to create client: %v", err)
//	}
func NewLumenClient(cfg *config.Config, logger *zap.Logger) (*LumenClient, error) {
	return NewLumenClientWithBalancer(cfg, logger, RoundRobin)
}

// GetConfig returns a copy of the client's current configuration in a thread-safe manner.
//
// This method allows REST handlers and other components to safely retrieve the runtime
// configuration without modifying the client's internal state. The returned configuration
// is a shallow copy to prevent external modifications from affecting the client.
//
// Returns:
//   - *config.Config: A copy of the current configuration
//
// Role in project: Provides safe read access to client configuration for monitoring,
// debugging, and REST API endpoints that need to expose configuration details.
//
// Example:
//
//	client, _ := client.NewLumenClient(cfg, logger)
//	currentCfg := client.GetConfig()
//	fmt.Printf("REST port: %d\n", currentCfg.Server.REST.Port)
func (c *LumenClient) GetConfig() *config.Config {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.config == nil {
		// 返回默认配置的副本
		d := config.DefaultConfig()
		return d
	}
	// 返回一个拷贝，避免调用方能修改内部结构
	copyCfg := *c.config
	return &copyCfg
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

// Start initializes and starts all client subsystems.
//
// This method performs the following initialization steps:
//  1. Starts the mDNS service discovery to find ML nodes
//  2. Sets up node change monitoring callbacks
//  3. Launches the connection pool maintenance loop
//  4. Begins metrics collection
//
// The method is idempotent - calling it multiple times will return an error if already running.
// The provided context is used for cancellation of background goroutines.
//
// Parameters:
//   - ctx: Context for controlling the lifecycle of background operations
//
// Returns:
//   - error: Non-nil if the client is already running or if any subsystem fails to start
//
// Role in project: Activates the client and makes it ready to handle inference requests.
// Must be called before performing any inference operations.
//
// Example:
//
//	client, _ := client.NewLumenClient(cfg, logger)
//	ctx := context.Background()
//	if err := client.Start(ctx); err != nil {
//	    log.Fatalf("Failed to start client: %v", err)
//	}
//	defer client.Close()
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

// inferSingle 是原来单请求路径的实现（可复用）
func (c *LumenClient) inferSingle(ctx context.Context, req *pb.InferRequest) (*pb.InferResponse, error) {
	startTime := time.Now()
	c.incrementTotalRequests()

	node, err := c.balancer.SelectNode(ctx, req.Task)
	if err != nil {
		c.incrementFailedRequests()
		return nil, fmt.Errorf("failed to select node: %w", err)
	}

	node.IncrementConnections()
	defer node.DecrementConnections()

	conn, err := c.pool.GetConnection(node)
	if err != nil {
		c.incrementFailedRequests()
		return nil, fmt.Errorf("failed to get connection for node %s: %w", node.ID, err)
	}

	resp, err := conn.Infer(ctx, req)
	if err != nil {
		c.incrementFailedRequests()
		c.pool.ReturnConnection(node.ID, conn)
		return nil, fmt.Errorf("inference failed: %w", err)
	}

	// 归还连接并更新统计
	c.pool.ReturnConnection(node.ID, conn)
	c.incrementSuccessfulRequests()
	c.updateLatency(time.Since(startTime))
	return resp, nil
}

// inferStreamSingle 是原来单-chunk 流式路径（原先的 InferStream 实现）
// 返回 channel，并负责 connection 的 lifecycle
func (c *LumenClient) inferStreamSingle(ctx context.Context, req *pb.InferRequest) (<-chan *pb.InferResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	node, err := c.balancer.SelectNode(ctx, req.Task)
	if err != nil {
		return nil, fmt.Errorf("failed to select node: %w", err)
	}

	conn, err := c.pool.GetConnection(node)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection for node %s: %w", node.ID, err)
	}

	// 调用底层连接的流式方法（已有实现）
	respChan, err := conn.InferStream(ctx, req)
	if err != nil {
		c.pool.ReturnConnection(node.ID, conn)
		return nil, fmt.Errorf("stream inference failed: %w", err)
	}

	// 包装通道以便在 goroutine 结束时归还连接
	wrapped := make(chan *pb.InferResponse, 100)
	go func() {
		defer close(wrapped)
		defer c.pool.ReturnConnection(node.ID, conn)

		for resp := range respChan {
			wrapped <- resp
			if resp.IsFinal {
				break
			}
		}
	}()

	return wrapped, nil
}

// Infer performs a synchronous inference request with automatic payload chunking.
//
// This method handles the complexity of large payloads by automatically splitting them into
// chunks based on the client's configuration. For small payloads, it sends a single request.
// For large payloads, it uses bidirectional streaming internally and returns only the final
// aggregated response.
//
// The method provides a simple request-response interface that abstracts away:
//   - Automatic payload chunking for large data
//   - Node selection via load balancing
//   - Connection management and pooling
//   - Error handling and metrics collection
//
// Parameters:
//   - ctx: Context for request timeout and cancellation
//   - req: The inference request containing task, payload, and metadata
//
// Returns:
//   - *pb.InferResponse: The final inference result
//   - error: Non-nil if node selection, connection, or inference fails
//
// Role in project: Primary API for synchronous ML inference operations. This is the most
// commonly used method for applications that need immediate results (e.g., embedding generation,
// image classification).
//
// Example:
//
//	// Text embedding
//	textData := []byte("Hello, world!")
//	embeddingReq, _ := types.NewEmbeddingRequest(textData)
//	inferReq := types.NewInferRequest("text_embedding").
//	    WithCorrelationID("req-123").
//	    ForEmbedding(embeddingReq, "text_embedding").
//	    Build()
//
//	result, err := client.Infer(ctx, inferReq)
//	if err != nil {
//	    log.Fatalf("Inference failed: %v", err)
//	}
//
//	embeddingResp, _ := types.ParseInferResponse(result).AsEmbeddingResponse()
//	fmt.Printf("Embedding dimensions: %d\n", embeddingResp.DimValue())
func (c *LumenClient) Infer(ctx context.Context, req *pb.InferRequest) (*pb.InferResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	// 决定使用的 chunk 配置（从 c.config 获取）
	chCfg := c.config.Chunk
	chunks, err := ChunkPayload(req.Payload, chCfg)
	if err != nil {
		return nil, err
	}

	// 如果只有一个 chunk，调用 helper（与以前兼容）
	if len(chunks) == 1 {
		return c.inferSingle(ctx, req)
	}

	// 多个 chunk：使用同一节点的一个 bidi stream 发送所有 chunk，然后等待最终响应
	startTime := time.Now()
	c.incrementTotalRequests()

	node, err := c.balancer.SelectNode(ctx, req.Task)
	if err != nil {
		c.incrementFailedRequests()
		return nil, fmt.Errorf("failed to select node: %w", err)
	}

	node.IncrementConnections()
	defer node.DecrementConnections()

	conn, err := c.pool.GetConnection(node)
	if err != nil {
		c.incrementFailedRequests()
		return nil, fmt.Errorf("failed to get connection for node %s: %w", node.ID, err)
	}

	stream, err := conn.Client.Infer(ctx)
	if err != nil {
		c.incrementFailedRequests()
		c.pool.ReturnConnection(node.ID, conn)
		return nil, fmt.Errorf("failed to create inference stream: %w", err)
	}

	// using cancelable context to coordinate sender/receiver
	sendCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	sendErrCh := make(chan error, 1)

	// sender goroutine
	go func() {
		defer func() {
			_ = stream.CloseSend()
		}()
		var offset uint64
		total := uint64(len(chunks))
		for i, chunk := range chunks {
			select {
			case <-sendCtx.Done():
				sendErrCh <- sendCtx.Err()
				return
			default:
			}

			sendReq := &pb.InferRequest{
				CorrelationId: req.CorrelationId,
				Task:          req.Task,
				Payload:       chunk,
				PayloadMime:   req.PayloadMime,
				Seq:           uint64(i),
				Total:         total,
				Offset:        offset,
				Meta:          req.Meta,
			}
			if err := stream.Send(sendReq); err != nil {
				conn.mu.Lock()
				conn.ErrorCount++
				conn.mu.Unlock()
				// notify receiver about send failure
				sendErrCh <- err
				cancel()
				return
			}
			offset += uint64(len(chunk))
		}
		// indicate send completed successfully
		sendErrCh <- nil
	}()

	// receiver: 等待最终响应或发送失败
	var finalResp *pb.InferResponse
	for {
		resp, err := stream.Recv()
		if err != nil {
			// check send error
			select {
			case se := <-sendErrCh:
				if se != nil {
					c.incrementFailedRequests()
					c.pool.ReturnConnection(node.ID, conn)
					return nil, fmt.Errorf("send failed: %w", se)
				}
			default:
				// no send error reported yet
			}

			c.incrementFailedRequests()
			c.pool.ReturnConnection(node.ID, conn)
			return nil, fmt.Errorf("failed to receive response: %w", err)
		}

		if resp.IsFinal {
			finalResp = resp
			break
		}
		// ignore intermediate partials for synchronous Infer
	}

	// 成功
	conn.mu.Lock()
	conn.UsageCount++
	conn.LastUsed = time.Now()
	conn.mu.Unlock()
	c.incrementSuccessfulRequests()
	c.updateLatency(time.Since(startTime))
	c.pool.ReturnConnection(node.ID, conn)
	return finalResp, nil
}

// InferStream performs streaming inference with automatic payload chunking.
//
// This method returns a channel that yields multiple responses, including partial results
// and the final result. It's useful for tasks that produce incremental outputs or when
// you want to receive progress updates during long-running inference operations.
//
// The method handles:
//   - Automatic chunking of large payloads across the same bidirectional stream
//   - Progressive result delivery through a Go channel
//   - Proper connection lifecycle management
//   - Graceful error handling and cleanup
//
// The returned channel will be closed when either:
//   - A final response (IsFinal=true) is received
//   - An error occurs during streaming
//   - The context is cancelled
//
// Parameters:
//   - ctx: Context for request timeout and cancellation
//   - req: The inference request with task, payload, and metadata
//
// Returns:
//   - <-chan *pb.InferResponse: Read-only channel for receiving streaming responses
//   - error: Non-nil if initial setup fails (node selection or stream creation)
//
// Role in project: Provides streaming API for applications that need real-time updates
// or want to process results incrementally (e.g., real-time video processing, progressive
// face detection in large images).
//
// Example:
//
//	// Streaming classification
//	imageData, _ := os.ReadFile("large_image.jpg")
//	classReq, _ := types.NewClassificationRequest(imageData)
//	inferReq := types.NewInferRequest("classification_stream").
//	    ForClassification(classReq, "classification_stream").
//	    Build()
//
//	respChan, err := client.InferStream(ctx, inferReq)
//	if err != nil {
//	    log.Fatalf("Stream inference failed: %v", err)
//	}
//
//	for resp := range respChan {
//	    if resp.IsFinal {
//	        fmt.Println("Final result received")
//	        classResp, _ := types.ParseInferResponse(resp).AsClassificationResponse()
//	        fmt.Printf("Top label: %v\n", classResp.TopK(1))
//	    } else {
//	        fmt.Println("Partial result received")
//	    }
//	}
func (c *LumenClient) InferStream(ctx context.Context, req *pb.InferRequest) (<-chan *pb.InferResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	chCfg := c.config.Chunk
	chunks, err := ChunkPayload(req.Payload, chCfg)
	if err != nil {
		return nil, err
	}

	// 单 chunk：使用原来的单-stream helper
	if len(chunks) == 1 {
		return c.inferStreamSingle(ctx, req)
	}

	// 多 chunk：在同一连接/stream 上发送所有 chunk，并将接收到的响应转发到返回通道
	node, err := c.balancer.SelectNode(ctx, req.Task)
	if err != nil {
		return nil, fmt.Errorf("failed to select node: %w", err)
	}

	conn, err := c.pool.GetConnection(node)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection for node %s: %w", node.ID, err)
	}

	stream, err := conn.Client.Infer(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create inference stream: %w", err)
	}

	out := make(chan *pb.InferResponse, 100)

	sendCtx, cancel := context.WithCancel(ctx)

	// sender goroutine
	sendErrCh := make(chan error, 1)
	go func() {
		defer func() {
			_ = stream.CloseSend()
		}()
		var offset uint64
		total := uint64(len(chunks))
		for i, chunk := range chunks {
			select {
			case <-sendCtx.Done():
				sendErrCh <- sendCtx.Err()
				return
			default:
			}

			sendReq := &pb.InferRequest{
				CorrelationId: req.CorrelationId,
				Task:          req.Task,
				Payload:       chunk,
				PayloadMime:   req.PayloadMime,
				Seq:           uint64(i),
				Total:         total,
				Offset:        offset,
				Meta:          req.Meta,
			}
			if err := stream.Send(sendReq); err != nil {
				conn.mu.Lock()
				conn.ErrorCount++
				conn.mu.Unlock()
				sendErrCh <- err
				cancel()
				return
			}
			offset += uint64(len(chunk))
		}
		sendErrCh <- nil
	}()

	// receiver goroutine
	go func() {
		defer func() {
			cancel()
			close(out)
			c.pool.ReturnConnection(node.ID, conn)
		}()

		for {
			resp, err := stream.Recv()
			if err != nil {
				// if send reported an error, we may want to propagate it as a final Error response;
				// here we simply stop the stream.
				return
			}
			out <- resp
			if resp.IsFinal {
				return
			}
		}
	}()

	// monitor send errors in background to cancel if necessary
	go func() {
		if se := <-sendErrCh; se != nil {
			// send failed; cancel and ensure out eventually closes
			cancel()
		}
	}()

	return out, nil
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
