package client

import (
	"context"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/edwinzhancn/lumen-sdk/pkg/config"
	"github.com/edwinzhancn/lumen-sdk/pkg/discovery"
	sdktypes "github.com/edwinzhancn/lumen-sdk/pkg/types"
	pb "github.com/edwinzhancn/lumen-sdk/proto"

	"go.uber.org/zap"
)

// ClientMetrics is a lightweight metrics snapshot for monitoring.
type ClientMetrics struct {
	TotalNodes      int       `json:"total_nodes"`
	ActiveNodes     int       `json:"active_nodes"`
	TotalRequests   int64     `json:"total_requests"`
	SuccessRequests int64     `json:"success_requests"`
	FailedRequests  int64     `json:"failed_requests"`
	AverageLatency  int64     `json:"average_latency_ns"`
	ErrorRate       float64   `json:"error_rate"`
	LastUpdated     time.Time `json:"last_updated"`
}

// LumenClient provides inference access to ML nodes.
//
// It composes a NodeResolver (discovery) with a Pool (connection management).
// Inference requests are sent directly to the selected gRPC connection;
// there is no intermediate caching, health checking, or task availability polling.
type LumenClient struct {
	pool     *Pool
	resolver discovery.NodeResolver
	config   *config.Config
	logger   *zap.Logger

	cancel context.CancelFunc
	mu     sync.Mutex

	// metrics (atomic for lock-free read)
	totalReqs      atomic.Int64
	successReqs    atomic.Int64
	failedReqs     atomic.Int64
	totalLatencyNs atomic.Int64
}

// NewLumenClient creates a new LumenClient.
//
// The client selects a resolver backend based on cfg:
//   - If cfg.Discovery.MDNSEnabled is true, a mDNS resolver is used.
//   - Otherwise, if cfg.Discovery.HubURL is set, a Gateway push resolver is used.
func NewLumenClient(cfg *config.Config, logger *zap.Logger) (*LumenClient, error) {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	pool := NewPool(logger)

	var resolver discovery.NodeResolver
	if cfg.Discovery.Enabled && cfg.Discovery.MDNSEnabled {
		resolver = discovery.NewMDNSResolver(&cfg.Discovery, logger)
	}
	if resolver == nil && cfg.Discovery.Enabled && cfg.Discovery.HubURL != "" {
		resolver = discovery.NewPushResolver(cfg.Discovery.HubURL, logger)
	}
	if resolver == nil {
		return nil, fmt.Errorf("no discovery backend configured: enable mDNS or set hub_url")
	}

	return &LumenClient{
		pool:     pool,
		resolver: resolver,
		config:   cfg,
		logger:   logger,
	}, nil
}

// Start begins node discovery and connection management.
func (c *LumenClient) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	ctx, c.cancel = context.WithCancel(ctx)
	go c.pool.Watch(ctx, c.resolver)

	c.logger.Info("lumen client started")
	return nil
}

// Infer performs a synchronous inference request.
//
// Picks a healthy gRPC connection from the pool. On failure the connection
// is marked unhealthy immediately so the next Pick selects a different node.
func (c *LumenClient) Infer(ctx context.Context, req *pb.InferRequest) (*pb.InferResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}
	if err := sdktypes.ValidateTaskRequest(req); err != nil {
		return nil, err
	}

	start := time.Now()
	c.totalReqs.Add(1)

	nc, err := c.pool.Pick(req.Task)
	if err != nil {
		c.failedReqs.Add(1)
		return nil, fmt.Errorf("no node for task %s: %w", req.Task, err)
	}

	stream, err := nc.cli.Infer(ctx)
	if err != nil {
		c.pool.MarkUnhealthy(nc)
		return nil, fmt.Errorf("infer stream %s: %w", nc.id, err)
	}

	if err := stream.Send(req); err != nil {
		c.pool.MarkUnhealthy(nc)
		c.failedReqs.Add(1)
		return nil, fmt.Errorf("send to %s: %w", nc.id, err)
	}
	if err := stream.CloseSend(); err != nil {
		c.pool.MarkUnhealthy(nc)
		c.failedReqs.Add(1)
		return nil, fmt.Errorf("close send %s: %w", nc.id, err)
	}

	var responses []*pb.InferResponse
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			c.pool.MarkUnhealthy(nc)
			c.failedReqs.Add(1)
			return nil, fmt.Errorf("recv from %s: %w", nc.id, err)
		}
		responses = append(responses, resp)
		if resp.IsFinal {
			break
		}
	}

	finalResp, err := sdktypes.AssembleInferResponses(responses)
	if err != nil {
		c.failedReqs.Add(1)
		return nil, fmt.Errorf("assemble response: %w", err)
	}

	c.successReqs.Add(1)
	c.totalLatencyNs.Add(time.Since(start).Nanoseconds())
	return finalResp, nil
}

// InferStream performs a streaming inference request.
func (c *LumenClient) InferStream(ctx context.Context, req *pb.InferRequest) (<-chan *pb.InferResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}
	if err := sdktypes.ValidateTaskRequest(req); err != nil {
		return nil, err
	}
	nc, err := c.pool.Pick(req.Task)
	if err != nil {
		return nil, fmt.Errorf("no node for task %s: %w", req.Task, err)
	}

	stream, err := nc.cli.Infer(ctx)
	if err != nil {
		c.pool.MarkUnhealthy(nc)
		return nil, fmt.Errorf("infer stream %s: %w", nc.id, err)
	}

	if err := stream.Send(req); err != nil {
		c.pool.MarkUnhealthy(nc)
		return nil, fmt.Errorf("send to %s: %w", nc.id, err)
	}
	if err := stream.CloseSend(); err != nil {
		c.pool.MarkUnhealthy(nc)
		return nil, fmt.Errorf("close send %s: %w", nc.id, err)
	}

	respChan := make(chan *pb.InferResponse, 100)
	go func() {
		defer close(respChan)
		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				return
			}
			if err != nil {
				c.pool.MarkUnhealthy(nc)
				return
			}
			respChan <- resp
			if resp.IsFinal {
				return
			}
		}
	}()

	return respChan, nil
}

// GetConfig returns a thread-safe copy of the current configuration.
func (c *LumenClient) GetConfig() *config.Config {
	if c.config == nil {
		d := config.DefaultConfig()
		return d
	}
	copyCfg := *c.config
	return &copyCfg
}

// GetNodes returns summary descriptors for all pool connections.
func (c *LumenClient) GetNodes() []*discovery.NodeInfo {
	return c.pool.NodeInfos()
}

// GetMetrics returns real metrics from the current process since start.
func (c *LumenClient) GetMetrics() *ClientMetrics {
	s := c.pool.Stats()
	total := c.totalReqs.Load()
	success := c.successReqs.Load()
	failed := c.failedReqs.Load()
	latencyNs := c.totalLatencyNs.Load()

	var avgLatency int64
	var errorRate float64
	if total > 0 {
		avgLatency = latencyNs / total
		errorRate = float64(failed) / float64(total)
	}

	return &ClientMetrics{
		TotalNodes:      s.TotalConnections,
		ActiveNodes:     s.HealthyConnections,
		TotalRequests:   total,
		SuccessRequests: success,
		FailedRequests:  failed,
		AverageLatency:  avgLatency,
		ErrorRate:       errorRate,
		LastUpdated:     time.Now(),
	}
}

// PoolStats returns current pool statistics.
func (c *LumenClient) PoolStats() PoolStats {
	return c.pool.Stats()
}

// WatchNodes registers a callback that fires whenever the node list changes.
func (c *LumenClient) WatchNodes(cb func([]*discovery.NodeInfo)) {
	c.pool.OnNodesChanged(cb)
}

// Close stops discovery and closes all gRPC connections.
func (c *LumenClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cancel != nil {
		c.cancel()
	}
	return c.pool.Close()
}
