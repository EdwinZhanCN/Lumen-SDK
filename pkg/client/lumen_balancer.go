package client

import (
	"context"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/edwinzhancn/lumen-sdk/pkg/discovery"
	pb "github.com/edwinzhancn/lumen-sdk/proto"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// balancerOptions holds tunable parameters for the lumen balancer.
type balancerOptions struct {
	connectTimeout        time.Duration
	rediscoveryBackoffMin time.Duration
	rediscoveryBackoffMax time.Duration
}

var balancerSeq int64

// newLumenBalancerName registers a new balancer builder with a unique name and
// returns the name. The builder shares state with Pool via the nodeRegistry.
func newLumenBalancerName(registry *nodeRegistry, opts balancerOptions, logger *zap.Logger) string {
	id := atomic.AddInt64(&balancerSeq, 1)
	name := fmt.Sprintf("lumen_task_aware_%d", id)
	balancer.Register(&lumenBalancerBuilder{
		name:     name,
		registry: registry,
		opts:     opts,
		logger:   logger,
	})
	return name
}

// nodeRegistry is the shared state between the balancer and Pool. The balancer
// writes node states; Pool reads them for NodeInfos/Stats queries.
type nodeRegistry struct {
	mu        sync.RWMutex
	nodes     map[string]*registeredNode
	onChanged func()
}

type registeredNode struct {
	identity       discovery.NodeIdentity
	addr           string
	state          connectivity.State
	capabilities   []*pb.Capability
	tasks          []string
	hardFailures   int
	cooldownUntil  time.Time
	cooldown       time.Duration
	txt            map[string]string
	incompatible   bool
	incompatReason string
}

func (r *nodeRegistry) nodeInfos() []*discovery.NodeInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]*discovery.NodeInfo, 0, len(r.nodes))
	for _, rn := range r.nodes {
		availability := availabilityFromRegistered(rn)
		out = append(out, &discovery.NodeInfo{
			ID:                 rn.identity.Key(),
			Address:            rn.addr,
			Status:             availability.NodeStatus(),
			Availability:       availability,
			Metadata:           buildCapabilityMetadata(rn.capabilities),
			Models:             buildModelInfos(rn.capabilities),
			Tasks:              tasksToIOTasksFromCapabilities(rn.capabilities, rn.tasks),
			Capabilities:       discovery.CloneCapabilities(rn.capabilities),
			Version:            rn.txt["v"],
			Runtime:            rn.txt["runtime"],
			LastSeen:           time.Now(),
			Compatible:         !rn.incompatible,
			IncompatibleReason: rn.incompatReason,
		})
	}
	return out
}

func (r *nodeRegistry) stats() (total, healthy int) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	total = len(r.nodes)
	for _, rn := range r.nodes {
		if rn.incompatible {
			total--
			continue
		}
		if rn.state == connectivity.Ready {
			healthy++
		}
	}
	return
}

// --- Balancer Builder ---

type lumenBalancerBuilder struct {
	name     string
	registry *nodeRegistry
	opts     balancerOptions
	logger   *zap.Logger
}

func (b *lumenBalancerBuilder) Name() string { return b.name }

func (b *lumenBalancerBuilder) Build(cc balancer.ClientConn, _ balancer.BuildOptions) balancer.Balancer {
	return &lumenBalancer{
		cc:       cc,
		subConns: make(map[string]*subConnState),
		registry: b.registry,
		options:  b.opts,
		logger:   b.logger,
	}
}

// --- Balancer ---

type subConnState struct {
	sc             balancer.SubConn
	addr           resolver.Address
	identity       discovery.NodeIdentity
	state          connectivity.State
	capabilities   []*pb.Capability
	tasks          []string
	hardFailures   int
	cooldownUntil  time.Time
	cooldown       time.Duration
	txt            map[string]string
	capFetching    bool
	incompatible   bool
	incompatReason string
}

type lumenBalancer struct {
	cc       balancer.ClientConn
	mu       sync.Mutex
	subConns map[string]*subConnState
	registry *nodeRegistry
	options  balancerOptions
	logger   *zap.Logger
}

func (lb *lumenBalancer) UpdateClientConnState(state balancer.ClientConnState) error {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	activeKeys := make(map[string]bool, len(state.ResolverState.Addresses))

	for _, addr := range state.ResolverState.Addresses {
		attr, ok := getNodeAttr(addr)
		if !ok {
			continue
		}
		key := attr.Identity.Key()
		activeKeys[key] = true

		existing, exists := lb.subConns[key]
		if exists {
			if existing.addr.Addr != addr.Addr {
				existing.addr = addr
				if existing.sc != nil {
					lb.cc.UpdateAddresses(existing.sc, []resolver.Address{addr})
				}
			}
			existing.tasks = mergeTasks(existing.tasks, attr.Tasks)
			existing.txt = attr.Txt
			if existing.incompatible == attr.Compatible {
				lb.setCompatibilityLocked(existing, key, attr)
			} else if existing.incompatible {
				// Both sides incompatible; keep the freshest reason.
				existing.incompatReason = attr.IncompatReason
			}
			continue
		}

		scs := &subConnState{
			identity: attr.Identity,
			addr:     addr,
			state:    connectivity.Idle,
			tasks:    attr.Tasks,
			txt:      attr.Txt,
		}
		if !attr.Compatible {
			// Incompatible nodes stay visible in the registry but never get a
			// SubConn, so no RPC can be routed to them.
			scs.incompatible = true
			scs.incompatReason = attr.IncompatReason
			lb.subConns[key] = scs
			continue
		}
		sc, err := lb.cc.NewSubConn([]resolver.Address{addr}, balancer.NewSubConnOptions{
			StateListener: lb.makeStateListener(key),
		})
		if err != nil {
			lb.log().Warn("failed to create SubConn", zap.String("id", key), zap.Error(err))
			continue
		}
		scs.sc = sc
		lb.subConns[key] = scs
		sc.Connect()
	}

	for key, scs := range lb.subConns {
		if activeKeys[key] {
			continue
		}
		if scs.sc != nil {
			lb.cc.RemoveSubConn(scs.sc)
		}
		delete(lb.subConns, key)
	}

	lb.syncRegistryLocked()
	lb.rebuildPickerLocked()
	return nil
}

func (lb *lumenBalancer) makeStateListener(key string) func(balancer.SubConnState) {
	return func(state balancer.SubConnState) {
		lb.handleSubConnStateChange(key, state)
	}
}

// setCompatibilityLocked transitions a node between the compatible and
// incompatible sets. Compatible nodes get a SubConn (created on demand when
// they previously had none); incompatible nodes have theirs removed so they
// can never be picked.
func (lb *lumenBalancer) setCompatibilityLocked(scs *subConnState, key string, attr nodeAttr) {
	if attr.Compatible {
		scs.incompatible = false
		scs.incompatReason = ""
		if scs.sc == nil {
			sc, err := lb.cc.NewSubConn([]resolver.Address{scs.addr}, balancer.NewSubConnOptions{
				StateListener: lb.makeStateListener(key),
			})
			if err != nil {
				lb.log().Warn("failed to create SubConn for compatible node", zap.String("id", key), zap.Error(err))
				return
			}
			scs.sc = sc
			scs.state = connectivity.Idle
			scs.capabilities = nil
			scs.hardFailures = 0
			scs.cooldownUntil = time.Time{}
			scs.cooldown = 0
		}
		scs.sc.Connect()
		return
	}
	scs.incompatible = true
	scs.incompatReason = attr.IncompatReason
	if scs.sc != nil {
		lb.cc.RemoveSubConn(scs.sc)
		scs.sc = nil
		scs.state = connectivity.Idle
		scs.capabilities = nil
	}
}

func (lb *lumenBalancer) handleSubConnStateChange(key string, state balancer.SubConnState) {
	lb.mu.Lock()
	scs, ok := lb.subConns[key]
	if !ok || scs.incompatible {
		// Stale callback after removal, or a node that is no longer routable.
		lb.mu.Unlock()
		return
	}
	prevState := scs.state
	scs.state = state.ConnectivityState

	if state.ConnectivityState == connectivity.Ready && prevState != connectivity.Ready {
		scs.hardFailures = 0
		scs.cooldownUntil = time.Time{}
		scs.cooldown = 0
		// Publish the Ready node immediately (TXT task hints may already allow
		// routing); the capability fetch refines the task set asynchronously
		// and retries with backoff instead of giving up on one failure.
		if !scs.capFetching {
			scs.capFetching = true
			go lb.fetchCapabilitiesWithRetry(key, scs.addr.Addr)
		}
		lb.syncRegistryLocked()
		lb.rebuildPickerLocked()
		lb.mu.Unlock()
		return
	}

	if state.ConnectivityState == connectivity.TransientFailure {
		scs.hardFailures++
		if scs.hardFailures >= hardFailureThreshold {
			lb.startCooldownLocked(scs, time.Now())
		}
	}

	if state.ConnectivityState == connectivity.Idle {
		scs.sc.Connect()
	}

	lb.syncRegistryLocked()
	lb.rebuildPickerLocked()
	lb.mu.Unlock()
}

func (lb *lumenBalancer) startCooldownLocked(scs *subConnState, now time.Time) {
	next := lb.options.rediscoveryBackoffMin
	if scs.cooldown > 0 {
		next = scs.cooldown * 2
		if next > lb.options.rediscoveryBackoffMax {
			next = lb.options.rediscoveryBackoffMax
		}
	}
	scs.cooldown = next
	scs.cooldownUntil = now.Add(next)
}

func (lb *lumenBalancer) ResolverError(err error) {
	lb.log().Warn("resolver error", zap.Error(err))
}

func (lb *lumenBalancer) UpdateSubConnState(_ balancer.SubConn, _ balancer.SubConnState) {}

func (lb *lumenBalancer) Close() {}

func (lb *lumenBalancer) ExitIdle() {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	for _, scs := range lb.subConns {
		if scs.state == connectivity.Idle {
			scs.sc.Connect()
		}
	}
}

func (lb *lumenBalancer) rebuildPickerLocked() {
	now := time.Now()
	var ready []*subConnState
	var probes []*subConnState

	for _, scs := range lb.subConns {
		if scs.incompatible {
			// Incompatible nodes are visible but never routable.
			continue
		}
		switch {
		case scs.state == connectivity.Ready:
			if scs.cooldownUntil.IsZero() || now.After(scs.cooldownUntil) {
				ready = append(ready, scs)
			}
		case scs.state != connectivity.Shutdown && !scs.cooldownUntil.IsZero() && now.After(scs.cooldownUntil):
			probes = append(probes, scs)
		}
	}

	picker := &lumenPicker{
		ready:    ready,
		probes:   probes,
		balancer: lb,
	}

	var aggState connectivity.State
	switch {
	case len(ready) > 0:
		aggState = connectivity.Ready
	case len(lb.subConns) == 0:
		aggState = connectivity.Idle
	default:
		aggState = connectivity.Connecting
	}

	lb.cc.UpdateState(balancer.State{
		ConnectivityState: aggState,
		Picker:            picker,
	})
}

// syncRegistryLocked copies SubConn state into the shared nodeRegistry so
// Pool can read it without needing a reference to the balancer.
func (lb *lumenBalancer) syncRegistryLocked() {
	if lb.registry == nil {
		return
	}

	lb.registry.mu.Lock()
	lb.registry.nodes = make(map[string]*registeredNode, len(lb.subConns))
	for key, scs := range lb.subConns {
		lb.registry.nodes[key] = &registeredNode{
			identity:       scs.identity,
			addr:           scs.addr.Addr,
			state:          scs.state,
			capabilities:   scs.capabilities,
			tasks:          scs.tasks,
			hardFailures:   scs.hardFailures,
			cooldownUntil:  scs.cooldownUntil,
			cooldown:       scs.cooldown,
			txt:            scs.txt,
			incompatible:   scs.incompatible,
			incompatReason: scs.incompatReason,
		}
	}
	lb.registry.mu.Unlock()

	if lb.registry.onChanged != nil {
		go lb.registry.onChanged()
	}
}

const (
	capFetchBackoffMin = 1 * time.Second
	capFetchBackoffMax = 8 * time.Second
)

// fetchCapabilitiesWithRetry keeps trying to fetch node capabilities for as
// long as the SubConn stays Ready on the same address. Unbounded on purpose: a
// hub that binds its port before models are downloaded (control-plane-first
// startup) answers UNAVAILABLE for many minutes while the connection stays
// Ready, so giving up after a fixed attempt count would leave the node
// capability-less until an unrelated reconnect. It clears the per-node
// capFetching guard on exit so a later Ready transition can start a fresh
// fetch.
func (lb *lumenBalancer) fetchCapabilitiesWithRetry(key, addr string) {
	defer func() {
		lb.mu.Lock()
		if scs, ok := lb.subConns[key]; ok {
			scs.capFetching = false
		}
		lb.mu.Unlock()
	}()

	backoff := capFetchBackoffMin
	for attempt := 1; ; attempt++ {
		if lb.fetchCapabilitiesForNode(key, addr) {
			return
		}

		lb.mu.Lock()
		scs, ok := lb.subConns[key]
		stale := !ok || scs.state != connectivity.Ready || scs.addr.Addr != addr
		lb.mu.Unlock()
		if stale {
			return
		}
		if attempt%10 == 0 {
			lb.log().Warn("cap fetch: node still not reporting capabilities; retrying",
				zap.String("id", key),
				zap.Int("attempts", attempt),
			)
		}

		time.Sleep(backoff)
		backoff *= 2
		if backoff > capFetchBackoffMax {
			backoff = capFetchBackoffMax
		}
	}
}

// fetchCapabilitiesForNode performs one capability fetch. It reports success
// only when at least one capability was received; the caller owns retries.
func (lb *lumenBalancer) fetchCapabilitiesForNode(key, addr string) bool {
	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:    10 * time.Second,
			Timeout: 3 * time.Second,
		}),
	)
	if err != nil {
		lb.log().Warn("cap fetch: dial failed", zap.String("id", key), zap.Error(err))
		return false
	}
	defer conn.Close()

	cli := pb.NewInferenceClient(conn)
	stream, err := cli.StreamCapabilities(ctx, &emptypb.Empty{})
	if err != nil {
		// A node that does not implement the capability RPC speaks a different
		// protocol; it cannot be parsed or scheduled. Mark it incompatible once
		// instead of retrying forever.
		if status.Code(err) == codes.Unimplemented {
			lb.mu.Lock()
			if scs, ok := lb.subConns[key]; ok {
				scs.incompatible = true
				scs.incompatReason = "capability RPC not implemented; node does not speak the supported data-plane protocol"
			}
			lb.syncRegistryLocked()
			lb.rebuildPickerLocked()
			lb.mu.Unlock()
			lb.log().Info("node marked incompatible: capability RPC unimplemented", zap.String("id", key))
			return true
		}
		lb.log().Warn("cap fetch: stream failed", zap.String("id", key), zap.Error(err))
		return false
	}

	var caps []*pb.Capability
	for {
		cap, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			lb.log().Debug("cap fetch: recv failed", zap.String("id", key), zap.Error(err))
			break
		}
		if cap != nil {
			caps = append(caps, cap)
		}
	}
	if len(caps) == 0 {
		return false
	}

	tasks := tasksFromCapabilities(caps)
	compat := compatibilityFromCapabilities(caps)

	lb.mu.Lock()
	scs, ok := lb.subConns[key]
	if ok {
		scs.capabilities = caps
		scs.tasks = mergeTasks(scs.tasks, tasks)
		if !compat.compatible {
			scs.incompatible = true
			scs.incompatReason = compat.incompatReason()
		}
	}
	lb.syncRegistryLocked()
	lb.rebuildPickerLocked()
	lb.mu.Unlock()

	lb.log().Info("capabilities fetched",
		zap.String("id", key),
		zap.Strings("tasks", tasks),
	)
	return true
}

func (lb *lumenBalancer) log() *zap.Logger {
	if lb.logger != nil {
		return lb.logger
	}
	return zap.NewNop()
}

// --- helpers ---

func availabilityFromRegistered(rn *registeredNode) discovery.NodeAvailability {
	if rn.incompatible {
		return discovery.NodeAvailabilityIncompatible
	}
	switch rn.state {
	case connectivity.Ready:
		return discovery.NodeAvailabilityReady
	case connectivity.Connecting:
		return discovery.NodeAvailabilityConnecting
	case connectivity.Idle:
		return discovery.NodeAvailabilityResolving
	case connectivity.TransientFailure:
		if rn.hardFailures >= hardFailureThreshold {
			return discovery.NodeAvailabilityUnavailable
		}
		return discovery.NodeAvailabilityRediscovering
	default:
		return discovery.NodeAvailabilityUnknown
	}
}

func buildCapabilityMetadata(caps []*pb.Capability) map[string]interface{} {
	if len(caps) == 0 {
		return nil
	}
	metadata := make(map[string]interface{})
	for _, cap := range caps {
		if cap == nil {
			continue
		}
		for k, v := range cap.Extra {
			metadata[k] = v
		}
		if len(cap.Precisions) > 0 {
			metadata[cap.ServiceName+".precisions"] = append([]string(nil), cap.Precisions...)
		}
		if cap.MaxConcurrency > 0 {
			metadata[cap.ServiceName+".max_concurrency"] = cap.MaxConcurrency
		}
	}
	if len(metadata) == 0 {
		return nil
	}
	return metadata
}

func buildModelInfos(caps []*pb.Capability) []*discovery.ModelInfo {
	var models []*discovery.ModelInfo
	for _, cap := range caps {
		if cap == nil {
			continue
		}
		for _, modelID := range cap.ModelIds {
			models = append(models, &discovery.ModelInfo{
				ID:      modelID,
				Runtime: cap.Runtime,
			})
		}
	}
	return models
}

// --- Picker ---

type lumenPicker struct {
	ready    []*subConnState
	probes   []*subConnState
	rrIdx    int64
	balancer *lumenBalancer
}

func (p *lumenPicker) Pick(info balancer.PickInfo) (balancer.PickResult, error) {
	task := TaskFromContext(info.Ctx)
	now := time.Now()

	candidates := filterByTask(p.ready, task, false, now)
	if len(candidates) == 0 {
		candidates = filterByTask(p.probes, task, true, now)
	}
	if len(candidates) == 0 {
		if task != "" && !anySupportsTask(p.ready, task) && !anySupportsTask(p.probes, task) {
			return balancer.PickResult{}, fmt.Errorf("no node supports task %q", task)
		}
		return balancer.PickResult{}, balancer.ErrNoSubConnAvailable
	}

	idx := atomic.AddInt64(&p.rrIdx, 1)
	picked := candidates[idx%int64(len(candidates))]

	return balancer.PickResult{
		SubConn: picked.sc,
		Done:    p.makeDone(picked),
	}, nil
}

func (p *lumenPicker) makeDone(scs *subConnState) func(balancer.DoneInfo) {
	return func(info balancer.DoneInfo) {
		lb := p.balancer
		if info.Err == nil {
			lb.mu.Lock()
			scs.hardFailures = 0
			scs.cooldownUntil = time.Time{}
			scs.cooldown = 0
			lb.syncRegistryLocked()
			lb.mu.Unlock()
			return
		}
		if !shouldAffectNodeHealth(nil, info.Err) {
			return
		}
		lb.mu.Lock()
		scs.hardFailures++
		if scs.hardFailures >= hardFailureThreshold {
			lb.startCooldownLocked(scs, time.Now())
		}
		lb.syncRegistryLocked()
		lb.rebuildPickerLocked()
		lb.mu.Unlock()
	}
}

func filterByTask(candidates []*subConnState, task string, requireExpiredCooldown bool, now time.Time) []*subConnState {
	var out []*subConnState
	for _, scs := range candidates {
		if scs.incompatible {
			continue
		}
		if task != "" && !nodeSupportsTaskSlice(scs.tasks, task) {
			continue
		}
		if requireExpiredCooldown {
			if scs.cooldownUntil.IsZero() || now.Before(scs.cooldownUntil) {
				continue
			}
		} else if !scs.cooldownUntil.IsZero() && now.Before(scs.cooldownUntil) {
			continue
		}
		out = append(out, scs)
	}
	return out
}

func anySupportsTask(candidates []*subConnState, task string) bool {
	for _, scs := range candidates {
		if nodeSupportsTaskSlice(scs.tasks, task) {
			return true
		}
	}
	return false
}

func nodeSupportsTaskSlice(tasks []string, task string) bool {
	for _, t := range tasks {
		if t == task {
			return true
		}
	}
	return false
}
