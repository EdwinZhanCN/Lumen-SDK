package client

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/edwinzhancn/lumen-sdk/pkg/discovery"
	pb "github.com/edwinzhancn/lumen-sdk/proto"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// ErrNoAvailableNode is returned when no healthy connection exists.
var ErrNoAvailableNode = fmt.Errorf("no available node")

const hardFailureThreshold = 3

// PoolOptions controls operational session behavior.
type PoolOptions struct {
	ConnectTimeout        time.Duration
	RediscoveryBackoffMin time.Duration
	RediscoveryBackoffMax time.Duration
}

func (o PoolOptions) normalized() PoolOptions {
	if o.ConnectTimeout <= 0 {
		o.ConnectTimeout = 10 * time.Second
	}
	if o.RediscoveryBackoffMin <= 0 {
		o.RediscoveryBackoffMin = 10 * time.Second
	}
	if o.RediscoveryBackoffMax < o.RediscoveryBackoffMin {
		o.RediscoveryBackoffMax = 2 * time.Minute
	}
	return o
}

// Pool manages a single gRPC ClientConn with a custom resolver and task-aware
// balancer. Discovery events are fed through the resolver; the balancer creates
// one SubConn per node and routes RPCs based on the task set in the context.
type Pool struct {
	mu       sync.RWMutex
	conn     *grpc.ClientConn
	cli      pb.InferenceClient
	registry *nodeRegistry
	watchers []func([]*discovery.NodeInfo)

	logger  *zap.Logger
	options PoolOptions
}

// NewPool creates an empty connection pool.
func NewPool(logger *zap.Logger) *Pool {
	return NewPoolWithOptions(logger, PoolOptions{})
}

// NewPoolWithOptions creates a Pool. Call Connect to create the gRPC connection.
func NewPoolWithOptions(logger *zap.Logger, options PoolOptions) *Pool {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Pool{
		logger:  logger,
		options: options.normalized(),
	}
}

// Connect creates the gRPC ClientConn using the given resolver backend.
func (p *Pool) Connect(resolver discovery.NodeResolver) error {
	registry := &nodeRegistry{
		nodes: make(map[string]*registeredNode),
		onChanged: func() {
			p.notifyWatchers()
		},
	}

	opts := p.options
	balancerName := newLumenBalancerName(registry, balancerOptions{
		connectTimeout:        opts.ConnectTimeout,
		rediscoveryBackoffMin: opts.RediscoveryBackoffMin,
		rediscoveryBackoffMax: opts.RediscoveryBackoffMax,
	}, p.logger)

	rb := &lumenResolverBuilder{
		nodeResolver: resolver,
		logger:       p.logger,
	}

	svcCfg := fmt.Sprintf(`{"loadBalancingConfig": [{"%s": {}}]}`, balancerName)

	conn, err := grpc.NewClient(
		lumenScheme+":///cluster",
		grpc.WithResolvers(rb),
		grpc.WithDefaultServiceConfig(svcCfg),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:    10 * time.Second,
			Timeout: 3 * time.Second,
		}),
	)
	if err != nil {
		return fmt.Errorf("create gRPC client: %w", err)
	}

	p.mu.Lock()
	p.conn = conn
	p.cli = pb.NewInferenceClient(conn)
	p.registry = registry
	p.mu.Unlock()

	// grpc.NewClient is lazy — force eager resolver/balancer startup so
	// node discovery begins immediately rather than on the first RPC.
	conn.Connect()

	return nil
}

// Client returns the gRPC InferenceClient backed by the pool.
func (p *Pool) Client() pb.InferenceClient {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.cli
}

// PoolStats is a read-only snapshot of pool state.
type PoolStats struct {
	TotalConnections   int `json:"total_connections"`
	HealthyConnections int `json:"healthy_connections"`
}

// Stats returns current pool statistics.
func (p *Pool) Stats() PoolStats {
	p.mu.RLock()
	reg := p.registry
	p.mu.RUnlock()
	if reg == nil {
		return PoolStats{}
	}
	total, healthy := reg.stats()
	return PoolStats{
		TotalConnections:   total,
		HealthyConnections: healthy,
	}
}

// NodeInfos returns snapshot descriptors for all connections.
func (p *Pool) NodeInfos() []*discovery.NodeInfo {
	p.mu.RLock()
	reg := p.registry
	p.mu.RUnlock()
	if reg == nil {
		return nil
	}
	return reg.nodeInfos()
}

// OnNodesChanged registers a callback invoked whenever the node list changes.
func (p *Pool) OnNodesChanged(cb func([]*discovery.NodeInfo)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.watchers = append(p.watchers, cb)
}

func (p *Pool) notifyWatchers() {
	p.mu.RLock()
	if len(p.watchers) == 0 {
		p.mu.RUnlock()
		return
	}
	reg := p.registry
	watchers := make([]func([]*discovery.NodeInfo), len(p.watchers))
	copy(watchers, p.watchers)
	p.mu.RUnlock()

	if reg == nil {
		return
	}
	nodes := reg.nodeInfos()
	for _, w := range watchers {
		go w(nodes)
	}
}

// Close closes the gRPC connection and clears the pool.
func (p *Pool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.conn != nil {
		if err := p.conn.Close(); err != nil {
			p.logger.Error("failed to close connection", zap.Error(err))
		}
		p.conn = nil
		p.cli = nil
		p.registry = nil
	}
	p.logger.Info("pool closed")
	return nil
}

// --- helpers ---

func splitEndpoint(endpoint string) (string, int, error) {
	host, portString, err := net.SplitHostPort(endpoint)
	if err != nil {
		return "", 0, err
	}
	port, err := strconv.Atoi(portString)
	if err != nil {
		return "", 0, err
	}
	return host, port, nil
}

func copyStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func mergeTasks(current, incoming []string) []string {
	seen := make(map[string]struct{}, len(current)+len(incoming))
	out := make([]string, 0, len(current)+len(incoming))
	for _, task := range current {
		task = strings.TrimSpace(task)
		if task == "" {
			continue
		}
		if _, ok := seen[task]; ok {
			continue
		}
		seen[task] = struct{}{}
		out = append(out, task)
	}
	for _, task := range incoming {
		task = strings.TrimSpace(task)
		if task == "" {
			continue
		}
		if _, ok := seen[task]; ok {
			continue
		}
		seen[task] = struct{}{}
		out = append(out, task)
	}
	return out
}

func tasksFromCapabilities(caps []*pb.Capability) []string {
	var tasks []string
	for _, cap := range caps {
		for _, task := range cap.GetTasks() {
			if task.GetName() != "" {
				tasks = append(tasks, task.GetName())
			}
		}
	}
	return mergeTasks(nil, tasks)
}

func tasksToIOTasks(names []string) []*pb.IOTask {
	out := make([]*pb.IOTask, 0, len(names))
	for _, n := range names {
		out = append(out, &pb.IOTask{Name: n})
	}
	return out
}
