package client

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/edwinzhancn/lumen-sdk/pkg/discovery"
	pb "github.com/edwinzhancn/lumen-sdk/proto"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// ErrNoAvailableNode is returned when no healthy connection exists.
var ErrNoAvailableNode = fmt.Errorf("no available node")

// Pool manages gRPC connections to ML inference nodes.
//
// Connection health is driven entirely by gRPC's built-in connectivity state
// and KeepAlive. There are no timers, caches, health RPCs, or polling loops.
//
// The pool reacts to NodeEvent values from a NodeResolver: it dials on
// NodeAdded and closes on NodeRemoved. gRPC connectivity state transitions
// move connections between the healthy and unhealthy subsets.
type Pool struct {
	mu       sync.RWMutex
	conns    map[string]*nodeConn // all connections, keyed by node ID
	healthy  []*nodeConn          // subset of conns in connectivity.Ready
	watchers []func([]*discovery.NodeInfo)

	logger *zap.Logger
	rrIdx  int64 // round-robin index
}

type nodeConn struct {
	id    string
	addr  string
	tasks []string
	conn  *grpc.ClientConn
	cli   pb.InferenceClient
}

// NewPool creates an empty connection pool.
func NewPool(logger *zap.Logger) *Pool {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Pool{
		conns:  make(map[string]*nodeConn),
		logger: logger,
	}
}

// Watch consumes NodeEvent values from a resolver and manages connections.
// Blocks until ctx is cancelled. Call this in a goroutine.
func (p *Pool) Watch(ctx context.Context, resolver discovery.NodeResolver) {
	ch, err := resolver.Watch(ctx)
	if err != nil {
		p.logger.Error("resolver Watch failed", zap.Error(err))
		return
	}
	for ev := range ch {
		switch ev.Type {
		case discovery.NodeAdded:
			p.add(ctx, ev)
		case discovery.NodeRemoved:
			p.remove(ev.NodeID)
		}
	}
}

func (p *Pool) add(ctx context.Context, ev discovery.NodeEvent) {
	p.mu.RLock()
	if _, exists := p.conns[ev.NodeID]; exists {
		p.mu.RUnlock()
		return
	}
	p.mu.RUnlock()

	conn, err := grpc.NewClient(ev.Address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:    10 * time.Second,
			Timeout: 3 * time.Second,
		}),
	)
	if err != nil {
		p.logger.Warn("dial failed", zap.String("node_id", ev.NodeID), zap.String("addr", ev.Address), zap.Error(err))
		return
	}

	nc := &nodeConn{
		id:    ev.NodeID,
		addr:  ev.Address,
		tasks: ev.Tasks,
		conn:  conn,
		cli:   pb.NewInferenceClient(conn),
	}

	p.mu.Lock()
	p.conns[ev.NodeID] = nc
	p.healthy = append(p.healthy, nc)
	p.mu.Unlock()

	go p.monitorConnectivity(nc)

	p.notifyWatchers()

	p.logger.Info("node connected", zap.String("id", ev.NodeID), zap.String("addr", ev.Address))
}

func (p *Pool) remove(nodeID string) {
	p.mu.Lock()
	nc, ok := p.conns[nodeID]
	if !ok {
		p.mu.Unlock()
		return
	}
	delete(p.conns, nodeID)
	p.removeFromHealthyLocked(nc)
	p.mu.Unlock()

	_ = nc.conn.Close()

	p.notifyWatchers()

	p.logger.Info("node removed", zap.String("id", nodeID))
}

// monitorConnectivity blocks watching gRPC connectivity state changes.
// This replaces health-check timers and explicit Health RPCs.
func (p *Pool) monitorConnectivity(nc *nodeConn) {
	state := nc.conn.GetState()
	for nc.conn.WaitForStateChange(context.Background(), state) {
		state = nc.conn.GetState()

		switch state {
		case connectivity.Ready:
			p.markHealthy(nc)
		case connectivity.TransientFailure, connectivity.Shutdown:
			p.MarkUnhealthy(nc)
		case connectivity.Idle:
			// KeepAlive will trigger gRPC to transition Idle → Connecting → Ready
		}
	}
}

func (p *Pool) markHealthy(nc *nodeConn) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, h := range p.healthy {
		if h.id == nc.id {
			return // already present
		}
	}
	p.healthy = append(p.healthy, nc)
	p.logger.Debug("node healthy", zap.String("id", nc.id))
}

// MarkUnhealthy removes the node from the healthy subset.
// Exported so callers can react to inference failures immediately,
// without waiting for the next connectivity state change.
func (p *Pool) MarkUnhealthy(nc *nodeConn) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.removeFromHealthyLocked(nc)
	p.logger.Debug("node marked unhealthy", zap.String("id", nc.id))
}

func (p *Pool) removeFromHealthyLocked(nc *nodeConn) {
	for i, h := range p.healthy {
		if h.id == nc.id {
			p.healthy[i] = p.healthy[len(p.healthy)-1]
			p.healthy = p.healthy[:len(p.healthy)-1]
			return
		}
	}
}

// Pick returns a healthy connection. Nodes that support preferredTask are
// prioritised. If no node explicitly advertises the task, any healthy node
// is returned.
func (p *Pool) Pick(preferredTask string) (*nodeConn, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(p.healthy) == 0 {
		return nil, ErrNoAvailableNode
	}

	candidates := p.healthy
	if preferredTask != "" {
		var filtered []*nodeConn
		for _, nc := range p.healthy {
			for _, t := range nc.tasks {
				if t == preferredTask {
					filtered = append(filtered, nc)
					break
				}
			}
		}
		if len(filtered) > 0 {
			candidates = filtered
		}
	}

	nc := candidates[atomic.AddInt64(&p.rrIdx, 1)%int64(len(candidates))]
	return nc, nil
}

// Client returns the gRPC InferenceClient for the given nodeConn.
func (nc *nodeConn) Client() pb.InferenceClient { return nc.cli }

// ID returns the node identifier.
func (nc *nodeConn) ID() string { return nc.id }

// PoolStats is a read-only snapshot of pool state.
type PoolStats struct {
	TotalConnections   int `json:"total_connections"`
	HealthyConnections int `json:"healthy_connections"`
}

// Stats returns current pool statistics.
func (p *Pool) Stats() PoolStats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return PoolStats{
		TotalConnections:   len(p.conns),
		HealthyConnections: len(p.healthy),
	}
}

// NodeInfos returns snapshot descriptors for all connections.
func (p *Pool) NodeInfos() []*discovery.NodeInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.nodeInfosLocked()
}

func (p *Pool) nodeInfosLocked() []*discovery.NodeInfo {
	out := make([]*discovery.NodeInfo, 0, len(p.conns))
	for _, nc := range p.conns {
		status := discovery.NodeStatusActive
		healthy := false
		for _, h := range p.healthy {
			if h.id == nc.id {
				healthy = true
				break
			}
		}
		if !healthy {
			status = discovery.NodeStatusError
		}
		out = append(out, &discovery.NodeInfo{
			ID:      nc.id,
			Address: nc.addr,
			Status:  status,
			Tasks:   tasksToIOTasks(nc.tasks),
		})
	}
	return out
}

// OnNodesChanged registers a callback invoked whenever the node list changes.
func (p *Pool) OnNodesChanged(cb func([]*discovery.NodeInfo)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.watchers = append(p.watchers, cb)
}

func tasksToIOTasks(names []string) []*pb.IOTask {
	out := make([]*pb.IOTask, 0, len(names))
	for _, n := range names {
		out = append(out, &pb.IOTask{Name: n})
	}
	return out
}

func (p *Pool) notifyWatchers() {
	p.mu.RLock()
	if len(p.watchers) == 0 {
		p.mu.RUnlock()
		return
	}
	nodes := p.nodeInfosLocked()
	watchers := make([]func([]*discovery.NodeInfo), len(p.watchers))
	copy(watchers, p.watchers)
	p.mu.RUnlock()
	for _, w := range watchers {
		go w(nodes)
	}
}

// Close closes all gRPC connections and clears the pool.
func (p *Pool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	for id, nc := range p.conns {
		if err := nc.conn.Close(); err != nil {
			p.logger.Error("failed to close connection", zap.String("id", id), zap.Error(err))
		}
	}
	p.conns = make(map[string]*nodeConn)
	p.healthy = nil
	p.logger.Info("pool closed")
	return nil
}
