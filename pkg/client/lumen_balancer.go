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

type protocolValidationState uint8

const (
	protocolValidationPending protocolValidationState = iota
	protocolValidationCompatible
	protocolValidationIncompatible
)

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
	validation     protocolValidationState
	incompatReason string
}

func (r *nodeRegistry) nodeInfos() []*discovery.NodeInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]*discovery.NodeInfo, 0, len(r.nodes))
	for _, rn := range r.nodes {
		availability := availabilityFromRegistered(rn)
		out = append(out, &discovery.NodeInfo{
			ID:           rn.identity.Key(),
			Address:      rn.addr,
			Status:       availability.NodeStatus(),
			Availability: availability,
			Metadata:     buildCapabilityMetadata(rn.capabilities),
			Models:       buildModelInfos(rn.capabilities),
			Tasks:        tasksToIOTasksFromCapabilities(rn.capabilities, rn.tasks),
			Capabilities: discovery.CloneCapabilities(rn.capabilities),
			Version:      rn.txt["v"],
			Runtime:      rn.txt["runtime"],
			LastSeen:     time.Now(),
			// Pending nodes are not yet known to be incompatible, but they are
			// still excluded from the picker until validation completes.
			Compatible:         rn.validation != protocolValidationIncompatible,
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
		if rn.validation == protocolValidationIncompatible {
			total--
			continue
		}
		if rn.state == connectivity.Ready && rn.validation == protocolValidationCompatible {
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
	validation     protocolValidationState
	validationGen  uint64
	capFetchGen    uint64
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
				lb.resetProtocolValidationLocked(existing)
				lb.cc.UpdateAddresses(existing.sc, []resolver.Address{addr})
				if existing.state == connectivity.Ready {
					lb.startCapabilityFetchLocked(key, existing)
				}
			}
			existing.txt = attr.Txt
			continue
		}

		scs := &subConnState{
			identity: attr.Identity,
			addr:     addr,
			state:    connectivity.Idle,
			txt:      attr.Txt,
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

func (lb *lumenBalancer) resetProtocolValidationLocked(scs *subConnState) {
	scs.validationGen++
	scs.validation = protocolValidationPending
	scs.incompatReason = ""
	scs.capabilities = nil
	scs.tasks = nil
}

func (lb *lumenBalancer) startCapabilityFetchLocked(key string, scs *subConnState) {
	if scs.capFetchGen == scs.validationGen {
		return
	}
	scs.capFetchGen = scs.validationGen
	go lb.fetchCapabilitiesWithRetry(key, scs.addr.Addr, scs.validationGen)
}

func (lb *lumenBalancer) handleSubConnStateChange(key string, state balancer.SubConnState) {
	lb.mu.Lock()
	scs, ok := lb.subConns[key]
	if !ok {
		// Stale callback after removal.
		lb.mu.Unlock()
		return
	}
	prevState := scs.state
	scs.state = state.ConnectivityState

	if state.ConnectivityState == connectivity.Ready && prevState != connectivity.Ready {
		// Every transport connection gets a fresh in-band verdict. Resolver
		// refreshes do not reset this state, but a genuine reconnect does.
		lb.resetProtocolValidationLocked(scs)
		scs.hardFailures = 0
		scs.cooldownUntil = time.Time{}
		scs.cooldown = 0
		// Pending nodes stay visible but are not routable until the capability
		// stream proves that they speak the supported protocol major.
		lb.startCapabilityFetchLocked(key, scs)
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

	if state.ConnectivityState == connectivity.Idle && scs.sc != nil {
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
		if scs.state == connectivity.Idle && scs.sc != nil {
			scs.sc.Connect()
		}
	}
}

func (lb *lumenBalancer) rebuildPickerLocked() {
	now := time.Now()
	var ready []*subConnState
	var probes []*subConnState
	var known []*subConnState
	hasPending := false

	for _, scs := range lb.subConns {
		switch scs.validation {
		case protocolValidationPending:
			if scs.state != connectivity.Shutdown {
				hasPending = true
			}
			continue
		case protocolValidationIncompatible:
			// Incompatible nodes are visible but never routable.
			continue
		}
		known = append(known, scs)
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
		ready:      ready,
		probes:     probes,
		known:      known,
		hasPending: hasPending,
		balancer:   lb,
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
			validation:     scs.validation,
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
// long as the same validation generation stays Ready on the same address.
// Unbounded on purpose: a
// hub that binds its port before models are downloaded (control-plane-first
// startup) answers UNAVAILABLE for many minutes while the connection stays
// Ready, so giving up after a fixed attempt count would leave the node
// capability-less until an unrelated reconnect. A generation token prevents
// a stale result from an old endpoint or transport from replacing a newer
// verdict.
func (lb *lumenBalancer) fetchCapabilitiesWithRetry(key, addr string, generation uint64) {
	defer func() {
		lb.mu.Lock()
		if scs, ok := lb.subConns[key]; ok && scs.capFetchGen == generation {
			scs.capFetchGen = 0
		}
		lb.mu.Unlock()
	}()

	backoff := capFetchBackoffMin
	for attempt := 1; ; attempt++ {
		if lb.fetchCapabilitiesForNode(key, addr, generation) {
			return
		}

		lb.mu.Lock()
		scs, ok := lb.subConns[key]
		stale := !ok || scs.state != connectivity.Ready || scs.addr.Addr != addr ||
			scs.validationGen != generation
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
func (lb *lumenBalancer) fetchCapabilitiesForNode(key, addr string, generation uint64) bool {
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
			lb.markCapabilityRPCUnimplemented(key, addr, generation)
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
			// Streaming status errors are commonly delivered by the first Recv
			// rather than by StreamCapabilities itself.
			if status.Code(err) == codes.Unimplemented {
				lb.markCapabilityRPCUnimplemented(key, addr, generation)
				return true
			}
			lb.log().Debug("cap fetch: recv failed", zap.String("id", key), zap.Error(err))
			return false
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
	applied := ok && scs.addr.Addr == addr && scs.validationGen == generation
	if applied {
		scs.capabilities = caps
		scs.tasks = mergeTasks(nil, tasks)
		if compat.compatible {
			scs.validation = protocolValidationCompatible
			scs.incompatReason = ""
		} else {
			scs.validation = protocolValidationIncompatible
			scs.incompatReason = compat.incompatReason()
		}
		lb.syncRegistryLocked()
		lb.rebuildPickerLocked()
	}
	lb.mu.Unlock()

	if applied {
		lb.log().Info("capabilities fetched",
			zap.String("id", key),
			zap.Strings("tasks", tasks),
		)
	}
	return true
}

func (lb *lumenBalancer) markCapabilityRPCUnimplemented(key, addr string, generation uint64) {
	lb.mu.Lock()
	scs, ok := lb.subConns[key]
	applied := ok && scs.addr.Addr == addr && scs.validationGen == generation
	if applied {
		scs.validation = protocolValidationIncompatible
		scs.capabilities = nil
		scs.tasks = nil
		scs.incompatReason = "capability RPC not implemented; node does not speak the supported data-plane protocol"
		lb.syncRegistryLocked()
		lb.rebuildPickerLocked()
	}
	lb.mu.Unlock()
	if applied {
		lb.log().Info("node marked incompatible: capability RPC unimplemented", zap.String("id", key))
	}
}

func (lb *lumenBalancer) log() *zap.Logger {
	if lb.logger != nil {
		return lb.logger
	}
	return zap.NewNop()
}

// --- helpers ---

func availabilityFromRegistered(rn *registeredNode) discovery.NodeAvailability {
	if rn.validation == protocolValidationIncompatible {
		return discovery.NodeAvailabilityIncompatible
	}
	switch rn.state {
	case connectivity.Ready:
		if rn.validation != protocolValidationCompatible {
			return discovery.NodeAvailabilityConnecting
		}
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
	ready      []*subConnState
	probes     []*subConnState
	known      []*subConnState
	hasPending bool
	rrIdx      int64
	balancer   *lumenBalancer
}

func (p *lumenPicker) Pick(info balancer.PickInfo) (balancer.PickResult, error) {
	task := TaskFromContext(info.Ctx)
	now := time.Now()

	candidates := filterByTask(p.ready, task, false, now)
	if len(candidates) == 0 {
		candidates = filterByTask(p.probes, task, true, now)
	}
	if len(candidates) == 0 {
		if p.hasPending {
			// At least one node is connected or connecting but has not completed
			// in-band validation. Ask gRPC to wait for the next picker update;
			// the RPC context bounds the wait.
			return balancer.PickResult{}, balancer.ErrNoSubConnAvailable
		}
		if task != "" && !anySupportsTask(p.known, task) {
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
		if scs.validation != protocolValidationCompatible {
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
		if scs.validation == protocolValidationCompatible && nodeSupportsTaskSlice(scs.tasks, task) {
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
