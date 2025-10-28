package client

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	pb "Lumen-SDK/proto"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

// GRPCConnectionPool gRPC连接池实现
type GRPCConnectionPool struct {
	config      *PoolConfig
	connections map[string]*Connection
	mu          sync.RWMutex
	logger      *zap.Logger
	running     bool
}

// Connection 连接信息
type Connection struct {
	Client      pb.InferenceClient
	Conn        *grpc.ClientConn
	Established time.Time
	LastUsed    time.Time
	UsageCount  int64
	ErrorCount  int64
	Status      ConnectionStatus
	mu          sync.RWMutex
}

// Infer 执行非流式推理请求（包装双向流）
func (conn *Connection) Infer(ctx context.Context, req *pb.InferRequest) (*pb.InferResponse, error) {
	// 创建双向流
	stream, err := conn.Client.Infer(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create inference stream: %w", err)
	}

	// 发送请求
	if err := stream.Send(req); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// 关闭发送方向
	if err := stream.CloseSend(); err != nil {
		return nil, fmt.Errorf("failed to close send: %w", err)
	}

	// 接收响应
	resp, err := stream.Recv()
	if err != nil {
		return nil, fmt.Errorf("failed to receive response: %w", err)
	}

	// 更新连接统计
	conn.mu.Lock()
	conn.UsageCount++
	conn.LastUsed = time.Now()
	conn.mu.Unlock()

	return resp, nil
}

// InferStream 执行流式推理请求
func (conn *Connection) InferStream(ctx context.Context, req *pb.InferRequest) (<-chan *pb.InferResponse, error) {
	// 创建双向流
	stream, err := conn.Client.Infer(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create inference stream: %w", err)
	}

	// 创建响应通道
	respChan := make(chan *pb.InferResponse, 100)

	// 启动goroutine处理流
	go func() {
		defer close(respChan)

		// 发送初始请求
		if err := stream.Send(req); err != nil {
			conn.mu.Lock()
			conn.ErrorCount++
			conn.mu.Unlock()
			return
		}

		// 接收响应
		for {
			resp, err := stream.Recv()
			if err != nil {
				break
			}

			respChan <- resp

			// 如果是最终响应，退出循环
			if resp.IsFinal {
				break
			}
		}

		// 更新连接统计
		conn.mu.Lock()
		conn.UsageCount++
		conn.LastUsed = time.Now()
		conn.mu.Unlock()
	}()

	return respChan, nil
}

// NewGRPCConnectionPool 创建新的gRPC连接池
func NewGRPCConnectionPool(config *PoolConfig, logger *zap.Logger) *GRPCConnectionPool {
	if config == nil {
		config = &PoolConfig{
			MaxConnections: 10,
			MaxIdleTime:    5 * time.Minute,
			MaxLifetime:    30 * time.Minute,
			ConnectionTTL:  10 * time.Minute,
			HealthCheck:    true,
			HealthInterval: 30 * time.Second,
		}
	}

	return &GRPCConnectionPool{
		config:      config,
		connections: make(map[string]*Connection),
		logger:      logger,
	}
}

// GetConnection 获取连接
func (p *GRPCConnectionPool) GetConnection(nodeID string) (*Connection, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	conn, exists := p.connections[nodeID]
	if !exists || !p.isConnectionHealthy(conn) {
		// 需要创建新连接
		newConn, err := p.createConnection(nodeID)
		if err != nil {
			return nil, fmt.Errorf("failed to create connection to %s: %w", nodeID, err)
		}
		conn = newConn
		p.connections[nodeID] = conn
	}

	// 更新使用信息
	conn.mu.Lock()
	conn.LastUsed = time.Now()
	conn.UsageCount++
	conn.Status = ConnectionStatusConnected
	conn.mu.Unlock()

	p.logger.Debug("retrieved connection from pool",
		zap.String("node_id", nodeID),
		zap.Int64("usage_count", conn.UsageCount),
	)

	return conn, nil
}

// ReturnConnection 归还连接
func (p *GRPCConnectionPool) ReturnConnection(nodeID string, conn *Connection) error {
	// 在这个实现中，连接由池管理，不需要显式归还
	// 但我们可以更新连接状态
	if conn != nil {
		conn.mu.Lock()
		conn.Status = ConnectionStatusConnected
		conn.mu.Unlock()

		// 这里可以通过服务发现减少节点的连接计数
		// 但由于连接池和节点管理分离，这个功能可以在上层实现
		p.logger.Debug("connection returned to pool",
			zap.String("node_id", nodeID))
	}

	return nil
}

// Close 关闭连接池
func (p *GRPCConnectionPool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.logger.Info("closing connection pool")

	var closeErrors []error

	for nodeID, conn := range p.connections {
		if conn.Conn != nil {
			if err := conn.Conn.Close(); err != nil {
				closeErrors = append(closeErrors, err)
				p.logger.Error("failed to close connection",
					zap.String("node_id", nodeID),
					zap.Error(err),
				)
			}
		}
	}

	p.connections = make(map[string]*Connection)
	p.running = false

	if len(closeErrors) > 0 {
		return fmt.Errorf("failed to close connections: %v", closeErrors)
	}

	p.logger.Info("connection pool closed successfully")
	return nil
}

// Stats 获取连接池统计信息
func (p *GRPCConnectionPool) Stats() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	totalConnections := len(p.connections)
	activeConnections := 0
	idleConnections := 0
	errorConnections := 0

	totalUsage := int64(0)
	totalErrors := int64(0)

	for _, conn := range p.connections {
		conn.mu.RLock()
		status := conn.Status
		totalUsage += conn.UsageCount
		totalErrors += conn.ErrorCount
		conn.mu.RUnlock()

		switch status {
		case ConnectionStatusConnected:
			activeConnections++
		case ConnectionStatusDisconnected:
			idleConnections++
		case ConnectionStatusError:
			errorConnections++
		}
	}

	return map[string]interface{}{
		"total_connections":  totalConnections,
		"active_connections": activeConnections,
		"idle_connections":   idleConnections,
		"error_connections":  errorConnections,
		"total_usage":        totalUsage,
		"total_errors":       totalErrors,
		"max_connections":    p.config.MaxConnections,
		"max_idle_time":      p.config.MaxIdleTime.String(),
		"max_lifetime":       p.config.MaxLifetime.String(),
	}
}

// createConnection 创建新连接
func (p *GRPCConnectionPool) createConnection(nodeID string) (*Connection, error) {
	p.logger.Debug("creating new connection",
		zap.String("node_id", nodeID),
	)

	// 这里需要从节点信息中获取地址
	// 在实际实现中，我们需要从ServiceDiscovery获取节点地址
	// 为了简化，我们假设nodeID包含了地址信息
	address := p.extractAddressFromNodeID(nodeID)
	if address == "" {
		return nil, fmt.Errorf("could not extract address from nodeID: %s", nodeID)
	}

	// 配置gRPC连接选项
	opts := []grpc.DialOption{
		grpc.WithInsecure(),
		grpc.WithBlock(),
		grpc.WithTimeout(5 * time.Second),
	}

	// 建立连接
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, address, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to dial %s: %w", address, err)
	}

	client := pb.NewInferenceClient(conn)

	connection := &Connection{
		Client:      client,
		Conn:        conn,
		Established: time.Now(),
		LastUsed:    time.Now(),
		UsageCount:  0,
		ErrorCount:  0,
		Status:      ConnectionStatusConnected,
	}

	p.logger.Info("created new connection",
		zap.String("node_id", nodeID),
		zap.String("address", address),
	)

	// 启动健康检查
	if p.config.HealthCheck {
		go p.healthCheckLoop(nodeID, connection)
	}

	return connection, nil
}

// isConnectionHealthy 检查连接是否健康
func (p *GRPCConnectionPool) isConnectionHealthy(conn *Connection) bool {
	if conn == nil {
		return false
	}

	conn.mu.RLock()
	defer conn.mu.RUnlock()

	// 检查连接状态
	if conn.Status == ConnectionStatusError {
		return false
	}

	// 检查连接是否过期
	if p.config.MaxLifetime > 0 && time.Since(conn.Established) > p.config.MaxLifetime {
		return false
	}

	// 检查连接是否空闲太久
	if p.config.MaxIdleTime > 0 && time.Since(conn.LastUsed) > p.config.MaxIdleTime {
		return false
	}

	return true
}

// healthCheckLoop 健康检查循环
func (p *GRPCConnectionPool) healthCheckLoop(nodeID string, conn *Connection) {
	if !p.config.HealthCheck {
		return
	}

	ticker := time.NewTicker(p.config.HealthInterval)
	defer ticker.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !p.performHealthCheck(nodeID, conn) {
				// 健康检查失败，标记连接为错误状态
				conn.mu.Lock()
				conn.Status = ConnectionStatusError
				conn.ErrorCount++
				conn.mu.Unlock()

				p.logger.Warn("health check failed for connection",
					zap.String("node_id", nodeID),
				)
			}
		}
	}
}

// performHealthCheck 执行健康检查
func (p *GRPCConnectionPool) performHealthCheck(nodeID string, conn *Connection) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 调用健康检查RPC
	_, err := conn.Client.Health(ctx, &emptypb.Empty{})
	if err != nil {
		p.logger.Debug("health check error",
			zap.String("node_id", nodeID),
			zap.Error(err),
		)
		return false
	}

	return true
}

// cleanupIdleConnections 清理空闲连接
func (p *GRPCConnectionPool) cleanupIdleConnections() {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	var toRemove []string

	for nodeID, conn := range p.connections {
		if p.config.MaxIdleTime > 0 && now.Sub(conn.LastUsed) > p.config.MaxIdleTime {
			toRemove = append(toRemove, nodeID)
		}
	}

	for _, nodeID := range toRemove {
		if conn := p.connections[nodeID]; conn != nil {
			if conn.Conn != nil {
				conn.Conn.Close()
			}
			delete(p.connections, nodeID)

			p.logger.Debug("removed idle connection",
				zap.String("node_id", nodeID),
			)
		}
	}
}

// EnsureConnection 确保连接存在
func (p *GRPCConnectionPool) EnsureConnection(nodeID, address string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 检查连接是否已存在且健康
	if conn, exists := p.connections[nodeID]; exists && p.isConnectionHealthy(conn) {
		return nil
	}

	// 创建新连接
	conn, err := p.createConnectionWithAddress(nodeID, address)
	if err != nil {
		return fmt.Errorf("failed to create connection for %s: %w", nodeID, err)
	}

	p.connections[nodeID] = conn
	p.logger.Debug("ensured connection",
		zap.String("node_id", nodeID),
		zap.String("address", address))

	return nil
}

// RemoveConnection 移除连接
func (p *GRPCConnectionPool) RemoveConnection(nodeID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if conn, exists := p.connections[nodeID]; exists {
		if conn.Conn != nil {
			if err := conn.Conn.Close(); err != nil {
				p.logger.Error("failed to close connection during removal",
					zap.String("node_id", nodeID),
					zap.Error(err))
			}
		}
		delete(p.connections, nodeID)
		p.logger.Debug("removed connection", zap.String("node_id", nodeID))
	}

	return nil
}

// createConnectionWithAddress 使用指定地址创建连接
func (p *GRPCConnectionPool) createConnectionWithAddress(nodeID, address string) (*Connection, error) {
	p.logger.Debug("creating new connection with address",
		zap.String("node_id", nodeID),
		zap.String("address", address),
	)

	// 配置gRPC连接选项
	opts := []grpc.DialOption{
		grpc.WithInsecure(),
		grpc.WithBlock(),
		grpc.WithTimeout(5 * time.Second),
	}

	// 建立连接
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, address, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to dial %s: %w", address, err)
	}

	client := pb.NewInferenceClient(conn)

	connection := &Connection{
		Client:      client,
		Conn:        conn,
		Established: time.Now(),
		LastUsed:    time.Now(),
		UsageCount:  0,
		ErrorCount:  0,
		Status:      ConnectionStatusConnected,
	}

	p.logger.Info("created new connection",
		zap.String("node_id", nodeID),
		zap.String("address", address),
	)

	// 启动健康检查
	if p.config.HealthCheck {
		go p.healthCheckLoop(nodeID, connection)
	}

	return connection, nil
}

// extractAddressFromNodeID 从节点ID中提取地址
func (p *GRPCConnectionPool) extractAddressFromNodeID(nodeID string) string {
	// 简化实现：假设nodeID格式为 "instance@address"
	parts := strings.Split(nodeID, "@")
	if len(parts) >= 2 {
		return parts[1]
	}

	// 或者直接返回nodeID作为地址
	return nodeID
}

// StartMaintenanceLoop 启动维护循环
func (p *GRPCConnectionPool) StartMaintenanceLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Minute) // 每分钟执行一次维护
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.cleanupIdleConnections()
		}
	}
}

// SimpleConnectionPool 简单连接池实现（用于测试）
type SimpleConnectionPool struct {
	connections map[string]pb.InferenceClient
	mu          sync.RWMutex
	logger      *zap.Logger
}

// NewSimpleConnectionPool 创建简单连接池
func NewSimpleConnectionPool(logger *zap.Logger) *SimpleConnectionPool {
	return &SimpleConnectionPool{
		connections: make(map[string]pb.InferenceClient),
		logger:      logger,
	}
}

// GetConnection 获取连接
func (p *SimpleConnectionPool) GetConnection(nodeID string) (pb.InferenceClient, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	client, exists := p.connections[nodeID]
	if !exists {
		return nil, fmt.Errorf("no connection found for node: %s", nodeID)
	}

	return client, nil
}

// ReturnConnection 归还连接
func (p *SimpleConnectionPool) ReturnConnection(nodeID string, client pb.InferenceClient) error {
	return nil // 简单实现不需要归还
}

// Close 关闭连接池
func (p *SimpleConnectionPool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.connections = make(map[string]pb.InferenceClient)
	p.logger.Info("simple connection pool closed")
	return nil
}

// Stats 获取统计信息
func (p *SimpleConnectionPool) Stats() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return map[string]interface{}{
		"total_connections": len(p.connections),
		"type":              "simple",
	}
}

// AddConnection 手动添加连接
func (p *SimpleConnectionPool) AddConnection(nodeID string, client pb.InferenceClient) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.connections[nodeID] = client
	p.logger.Debug("added connection to simple pool",
		zap.String("node_id", nodeID),
	)
}
