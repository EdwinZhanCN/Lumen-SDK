package client

import (
	"context"
	"errors"
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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// shouldAffectNodeHealth reports whether an inference error should count
// against the selected node's pool health.
func shouldAffectNodeHealth(ctx context.Context, err error) bool {
	if err == nil {
		return false
	}
	if ctx != nil && ctx.Err() != nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	st, ok := status.FromError(err)
	if ok {
		switch st.Code() {
		case codes.Unavailable:
			return true
		case codes.Canceled,
			codes.DeadlineExceeded,
			codes.InvalidArgument,
			codes.NotFound,
			codes.AlreadyExists,
			codes.PermissionDenied,
			codes.Unauthenticated,
			codes.FailedPrecondition,
			codes.ResourceExhausted,
			codes.OutOfRange,
			codes.Unimplemented,
			codes.Internal:
			return false
		default:
			return false
		}
	}

	return true
}

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
// It composes a NodeResolver (discovery) with a Pool (gRPC connection pool).
// The Pool uses a custom gRPC resolver and task-aware balancer so that RPCs
// are routed to the correct node based on the requested inference task.
type LumenClient struct {
	pool     *Pool
	resolver discovery.NodeResolver
	config   *config.Config
	logger   *zap.Logger

	cancel context.CancelFunc
	mu     sync.Mutex

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

	pool := NewPoolWithOptions(logger, PoolOptions{
		ConnectTimeout:        cfg.Discovery.ConnectTimeout,
		RediscoveryBackoffMin: cfg.Discovery.RediscoveryBackoffMin,
		RediscoveryBackoffMax: cfg.Discovery.RediscoveryBackoffMax,
	})

	var resolver discovery.NodeResolver
	if cfg.Discovery.Enabled && cfg.Discovery.MDNSEnabled {
		resolver = discovery.NewMDNSResolver(&cfg.Discovery, logger)
	}
	if resolver == nil && cfg.Discovery.Enabled && cfg.Discovery.HubURL != "" {
		resolver = discovery.NewPushResolverWithDeployment(cfg.Discovery.HubURL, cfg.Discovery.DeploymentID, logger)
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
// It blocks until at least one node has reported its capabilities,
// or until ctx is cancelled / the connect timeout elapses.
func (c *LumenClient) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	_, c.cancel = context.WithCancel(ctx)

	ready := make(chan struct{}, 1)
	c.pool.OnNodesChanged(func(nodes []*discovery.NodeInfo) {
		for _, n := range nodes {
			if n.IsActive() && len(n.Tasks) > 0 {
				select {
				case ready <- struct{}{}:
				default:
				}
				return
			}
		}
	})

	if err := c.pool.Connect(c.resolver); err != nil {
		return fmt.Errorf("pool connect: %w", err)
	}

	timeout := c.config.Discovery.ConnectTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	select {
	case <-ready:
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(timeout):
		c.logger.Warn("timed out waiting for node capabilities, continuing anyway")
	}

	c.logger.Info("lumen client started")
	return nil
}

// Infer performs a synchronous inference request.
//
// The task is set in the context so the balancer's Picker routes the RPC to a
// node that supports the requested task. Health feedback is handled
// automatically by the Picker's Done callback.
func (c *LumenClient) Infer(ctx context.Context, req *pb.InferRequest) (*pb.InferResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}
	if err := sdktypes.ValidateTaskRequest(req); err != nil {
		return nil, err
	}

	c.resolveService(req)

	start := time.Now()
	c.totalReqs.Add(1)

	chunks, err := ChunkPayload(req.Payload, c.config.Chunk)
	if err != nil {
		c.failedReqs.Add(1)
		return nil, fmt.Errorf("chunk payload: %w", err)
	}

	cli := c.pool.Client()
	if cli == nil {
		c.failedReqs.Add(1)
		return nil, ErrNoAvailableNode
	}

	ctx = WithTask(ctx, req.Task)

	if len(chunks) == 1 {
		resp, err := c.inferSingle(ctx, cli, req)
		if err != nil {
			c.failedReqs.Add(1)
			return nil, err
		}
		c.successReqs.Add(1)
		c.totalLatencyNs.Add(time.Since(start).Nanoseconds())
		return resp, nil
	}

	stream, err := cli.Infer(ctx)
	if err != nil {
		c.failedReqs.Add(1)
		return nil, fmt.Errorf("infer stream: %w", err)
	}

	sendCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	sendErrCh := make(chan error, 1)
	go func() {
		defer func() { _ = stream.CloseSend() }()
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
				sendErrCh <- err
				cancel()
				return
			}
			offset += uint64(len(chunk))
		}
		sendErrCh <- nil
	}()

	var responses []*pb.InferResponse
	for {
		resp, err := stream.Recv()
		if err != nil {
			if err == io.EOF && len(responses) > 0 {
				break
			}
			c.failedReqs.Add(1)
			select {
			case se := <-sendErrCh:
				if se != nil {
					return nil, fmt.Errorf("send failed: %w", se)
				}
			default:
			}
			return nil, fmt.Errorf("recv: %w", err)
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

func (c *LumenClient) inferSingle(ctx context.Context, cli pb.InferenceClient, req *pb.InferRequest) (*pb.InferResponse, error) {
	stream, err := cli.Infer(ctx)
	if err != nil {
		return nil, fmt.Errorf("infer stream: %w", err)
	}

	if err := stream.Send(req); err != nil {
		return nil, fmt.Errorf("send: %w", err)
	}
	if err := stream.CloseSend(); err != nil {
		return nil, fmt.Errorf("close send: %w", err)
	}

	var responses []*pb.InferResponse
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("recv: %w", err)
		}
		responses = append(responses, resp)
		if resp.IsFinal {
			break
		}
	}

	return sdktypes.AssembleInferResponses(responses)
}

// InferStream performs a streaming inference request.
func (c *LumenClient) InferStream(ctx context.Context, req *pb.InferRequest) (<-chan *pb.InferResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}
	if err := sdktypes.ValidateTaskRequest(req); err != nil {
		return nil, err
	}

	cli := c.pool.Client()
	if cli == nil {
		return nil, ErrNoAvailableNode
	}

	ctx = WithTask(ctx, req.Task)

	stream, err := cli.Infer(ctx)
	if err != nil {
		return nil, fmt.Errorf("infer stream: %w", err)
	}

	if err := stream.Send(req); err != nil {
		return nil, fmt.Errorf("send: %w", err)
	}
	if err := stream.CloseSend(); err != nil {
		return nil, fmt.Errorf("close send: %w", err)
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

// resolveService auto-fills req.Meta["service"] from node capabilities
// when the caller didn't specify one and the task maps to a single service.
func (c *LumenClient) resolveService(req *pb.InferRequest) {
	if sdktypes.ServiceFromMeta(req.Meta) != "" {
		return
	}
	for _, node := range c.pool.NodeInfos() {
		if services := node.MatchingServices(req.Task); len(services) == 1 {
			if req.Meta == nil {
				req.Meta = make(map[string]string)
			}
			req.Meta[sdktypes.MetaService] = services[0]
			return
		}
	}
}

// Close stops discovery and closes all connections.
func (c *LumenClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cancel != nil {
		c.cancel()
	}
	return c.pool.Close()
}
