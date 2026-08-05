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
	connectTimeout     time.Duration
	failureCooldownMin time.Duration
	failureCooldownMax time.Duration
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
	metadata       map[string]string
	compatibility  discovery.CompatibilityState
	incompatReason string
	updatedAt      time.Time
}

func (r *nodeRegistry) nodeInfos() []*discovery.NodeInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]*discovery.NodeInfo, 0, len(r.nodes))
	for _, node := range r.nodes {
		out = append(out, &discovery.NodeInfo{
			ID:                 node.identity.Key(),
			Address:            node.addr,
			Availability:       availabilityFromRegistered(node),
			Compatibility:      node.compatibility,
			Metadata:           buildCapabilityMetadata(node.capabilities),
			Models:             buildModelInfos(node.capabilities),
			Capabilities:       discovery.CloneCapabilities(node.capabilities),
			Version:            node.metadata["v"],
			Runtime:            node.metadata["runtime"],
			UpdatedAt:          node.updatedAt,
			IncompatibleReason: node.incompatReason,
		})
	}
	return out
}

func (r *nodeRegistry) stats() (total, routable int) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	total = len(r.nodes)
	for _, node := range r.nodes {
		if availabilityFromRegistered(node) == discovery.NodeAvailabilityReady &&
			node.compatibility == discovery.CompatibilityCompatible {
			routable++
		}
	}
	return total, routable
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
	sc                 balancer.SubConn
	generation         uint64
	addr               resolver.Address
	identity           discovery.NodeIdentity
	state              connectivity.State
	capabilities       []*pb.Capability
	hardFailures       int
	cooldownUntil      time.Time
	cooldown           time.Duration
	cooldownTimer      *time.Timer
	metadata           map[string]string
	compatibility      discovery.CompatibilityState
	compatibilityGen   uint64
	capabilityFetchGen uint64
	incompatReason     string
	updatedAt          time.Time
}

type lumenBalancer struct {
	cc             balancer.ClientConn
	mu             sync.Mutex
	subConns       map[string]*subConnState
	registry       *nodeRegistry
	options        balancerOptions
	logger         *zap.Logger
	nextSubConnGen uint64
	closed         bool
}

func (lb *lumenBalancer) UpdateClientConnState(state balancer.ClientConnState) error {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	if lb.closed {
		return nil
	}

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
				lb.resetCompatibilityLocked(existing)
				lb.cc.UpdateAddresses(existing.sc, []resolver.Address{addr})
				if existing.state == connectivity.Ready {
					lb.startCapabilityFetchLocked(key, existing)
				}
			}
			existing.metadata = copyStringMap(attr.Metadata)
			existing.updatedAt = time.Now()
			continue
		}

		lb.nextSubConnGen++
		generation := lb.nextSubConnGen
		scs := &subConnState{
			generation:    generation,
			identity:      attr.Identity,
			addr:          addr,
			state:         connectivity.Idle,
			metadata:      copyStringMap(attr.Metadata),
			compatibility: discovery.CompatibilityPending,
			updatedAt:     time.Now(),
		}
		sc, err := lb.cc.NewSubConn([]resolver.Address{addr}, balancer.NewSubConnOptions{
			StateListener: lb.makeStateListener(key, generation),
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
		if scs.cooldownTimer != nil {
			scs.cooldownTimer.Stop()
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

func (lb *lumenBalancer) makeStateListener(key string, generation uint64) func(balancer.SubConnState) {
	return func(state balancer.SubConnState) {
		lb.handleSubConnStateChange(key, generation, state)
	}
}

func (lb *lumenBalancer) resetCompatibilityLocked(scs *subConnState) {
	scs.compatibilityGen++
	scs.compatibility = discovery.CompatibilityPending
	scs.incompatReason = ""
	scs.capabilities = nil
	scs.updatedAt = time.Now()
}

func (lb *lumenBalancer) startCapabilityFetchLocked(key string, scs *subConnState) {
	if scs.capabilityFetchGen == scs.compatibilityGen && scs.capabilityFetchGen != 0 {
		return
	}
	scs.capabilityFetchGen = scs.compatibilityGen
	go lb.fetchCapabilitiesWithRetry(key, scs.addr.Addr, scs.compatibilityGen)
}

func (lb *lumenBalancer) handleSubConnStateChange(key string, generation uint64, state balancer.SubConnState) {
	lb.mu.Lock()
	scs, ok := lb.subConns[key]
	if lb.closed || !ok || scs.generation != generation {
		// A removed SubConn can report one final state after the same identity has
		// already been re-added. Generation matching prevents that stale callback
		// from mutating the replacement session.
		lb.mu.Unlock()
		return
	}
	prevState := scs.state
	scs.state = state.ConnectivityState
	scs.updatedAt = time.Now()

	if state.ConnectivityState == connectivity.Ready && prevState != connectivity.Ready {
		// Every transport connection gets a fresh in-band verdict. Resolver
		// refreshes do not reset this state, but a genuine reconnect does.
		lb.resetCompatibilityLocked(scs)
		scs.hardFailures = 0
		if scs.cooldownTimer != nil {
			scs.cooldownTimer.Stop()
			scs.cooldownTimer = nil
		}
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
			lb.startCooldownLocked(key, scs, time.Now())
		}
	}

	if state.ConnectivityState == connectivity.Idle && scs.sc != nil {
		scs.sc.Connect()
	}

	lb.syncRegistryLocked()
	lb.rebuildPickerLocked()
	lb.mu.Unlock()
}

func (lb *lumenBalancer) startCooldownLocked(key string, scs *subConnState, now time.Time) {
	next := lb.options.failureCooldownMin
	if scs.cooldown > 0 {
		next = scs.cooldown * 2
		if next > lb.options.failureCooldownMax {
			next = lb.options.failureCooldownMax
		}
	}
	scs.cooldown = next
	deadline := now.Add(next)
	scs.cooldownUntil = deadline
	if scs.cooldownTimer != nil {
		scs.cooldownTimer.Stop()
		scs.cooldownTimer = nil
	}
	if key == "" || lb.closed {
		return
	}
	scs.cooldownTimer = time.AfterFunc(next, func() {
		lb.mu.Lock()
		defer lb.mu.Unlock()
		current, ok := lb.subConns[key]
		if lb.closed || !ok || current != scs || !current.cooldownUntil.Equal(deadline) {
			return
		}
		current.cooldownTimer = nil
		// A Picker that returned ErrNoSubConnAvailable remains queued until gRPC
		// receives a state update. Rebuild exactly at cooldown expiry so existing
		// RPCs can probe the node again without requiring an unrelated event.
		lb.rebuildPickerLocked()
	})
}

func (lb *lumenBalancer) ResolverError(err error) {
	lb.log().Warn("resolver error", zap.Error(err))
}

func (lb *lumenBalancer) UpdateSubConnState(_ balancer.SubConn, _ balancer.SubConnState) {}

func (lb *lumenBalancer) Close() {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.closed = true
	for _, scs := range lb.subConns {
		if scs.cooldownTimer != nil {
			scs.cooldownTimer.Stop()
			scs.cooldownTimer = nil
		}
	}
}

func (lb *lumenBalancer) ExitIdle() {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	if lb.closed {
		return
	}
	for _, scs := range lb.subConns {
		if scs.state == connectivity.Idle && scs.sc != nil {
			scs.sc.Connect()
		}
	}
}

func (lb *lumenBalancer) rebuildPickerLocked() {
	now := time.Now()
	var ready []pickerNode
	var known []pickerNode
	hasPending := false

	for key, scs := range lb.subConns {
		switch scs.compatibility {
		case discovery.CompatibilityPending:
			if scs.state != connectivity.Shutdown {
				hasPending = true
			}
			continue
		case discovery.CompatibilityIncompatible:
			// Incompatible nodes are visible but never routable.
			continue
		}

		node := pickerNode{
			key:           key,
			sc:            scs.sc,
			generation:    scs.compatibilityGen,
			capabilities:  discovery.CloneCapabilities(scs.capabilities),
			cooldownUntil: scs.cooldownUntil,
		}
		known = append(known, node)
		switch {
		case scs.state == connectivity.Ready:
			if scs.cooldownUntil.IsZero() || now.After(scs.cooldownUntil) {
				ready = append(ready, node)
			}
		}
	}

	picker := &lumenPicker{
		ready:      ready,
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
			capabilities:   discovery.CloneCapabilities(scs.capabilities),
			metadata:       copyStringMap(scs.metadata),
			compatibility:  scs.compatibility,
			incompatReason: scs.incompatReason,
			updatedAt:      scs.updatedAt,
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
// long as the same compatibility generation stays Ready on the same address.
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
		if scs, ok := lb.subConns[key]; ok && scs.capabilityFetchGen == generation {
			scs.capabilityFetchGen = 0
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
			scs.compatibilityGen != generation
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
	ctx, cancel := context.WithTimeout(context.Background(), lb.options.connectTimeout)
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

	compatibility, reason := compatibilityFromCapabilities(caps)

	lb.mu.Lock()
	scs, ok := lb.subConns[key]
	applied := !lb.closed && ok && scs.addr.Addr == addr && scs.compatibilityGen == generation
	if applied {
		scs.capabilities = discovery.CloneCapabilities(caps)
		scs.compatibility = compatibility
		scs.incompatReason = reason
		scs.updatedAt = time.Now()
		lb.syncRegistryLocked()
		lb.rebuildPickerLocked()
	}
	lb.mu.Unlock()

	if applied {
		lb.log().Info("capabilities fetched",
			zap.String("id", key),
			zap.Strings("tasks", taskNamesFromCapabilities(caps)),
		)
	}
	return true
}

func (lb *lumenBalancer) markCapabilityRPCUnimplemented(key, addr string, generation uint64) {
	lb.mu.Lock()
	scs, ok := lb.subConns[key]
	applied := !lb.closed && ok && scs.addr.Addr == addr && scs.compatibilityGen == generation
	if applied {
		scs.compatibility = discovery.CompatibilityIncompatible
		scs.capabilities = nil
		scs.incompatReason = "capability RPC not implemented; node does not speak the supported data-plane protocol"
		scs.updatedAt = time.Now()
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

func availabilityFromRegistered(node *registeredNode) discovery.NodeAvailability {
	switch node.state {
	case connectivity.Idle:
		return discovery.NodeAvailabilityDiscovered
	case connectivity.Connecting:
		return discovery.NodeAvailabilityConnecting
	case connectivity.Ready:
		return discovery.NodeAvailabilityReady
	default:
		return discovery.NodeAvailabilityUnavailable
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

// pickerNode is an immutable projection built while the balancer lock is held.
// gRPC may keep an old Picker alive after a newer state is published, so a
// Picker must never retain pointers to mutable subConnState values.
type pickerNode struct {
	key           string
	sc            balancer.SubConn
	generation    uint64
	capabilities  []*pb.Capability
	cooldownUntil time.Time
}

type lumenPicker struct {
	ready      []pickerNode
	known      []pickerNode
	hasPending bool
	rrIdx      int64
	balancer   *lumenBalancer
}

func (p *lumenPicker) Pick(info balancer.PickInfo) (balancer.PickResult, error) {
	task := TaskFromContext(info.Ctx)
	now := time.Now()

	candidates := filterByTask(p.ready, task, now)
	if len(candidates) == 0 {
		if p.hasPending {
			// At least one node is connected or connecting but has not completed
			// in-band compatibility validation. Ask gRPC to wait for the next
			// picker update; the RPC context bounds the wait.
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

func (p *lumenPicker) makeDone(picked pickerNode) func(balancer.DoneInfo) {
	return func(info balancer.DoneInfo) {
		lb := p.balancer
		if info.Err != nil && !shouldAffectNodeHealth(nil, info.Err) {
			return
		}

		lb.mu.Lock()
		defer lb.mu.Unlock()
		scs, ok := lb.subConns[picked.key]
		if !ok || scs.sc != picked.sc || scs.compatibilityGen != picked.generation {
			// Completion from a removed/replaced endpoint must not mutate the
			// current node generation.
			return
		}

		if info.Err == nil {
			if scs.cooldownTimer != nil {
				scs.cooldownTimer.Stop()
				scs.cooldownTimer = nil
			}
			scs.hardFailures = 0
			scs.cooldownUntil = time.Time{}
			scs.cooldown = 0
		} else {
			scs.hardFailures++
			if scs.hardFailures >= hardFailureThreshold {
				lb.startCooldownLocked(picked.key, scs, time.Now())
			}
		}
		scs.updatedAt = time.Now()
		lb.syncRegistryLocked()
		lb.rebuildPickerLocked()
	}
}

func filterByTask(candidates []pickerNode, task string, now time.Time) []pickerNode {
	out := make([]pickerNode, 0, len(candidates))
	for _, candidate := range candidates {
		if task != "" && !capabilitiesSupportTask(candidate.capabilities, task) {
			continue
		}
		if !candidate.cooldownUntil.IsZero() && now.Before(candidate.cooldownUntil) {
			continue
		}
		out = append(out, candidate)
	}
	return out
}

func anySupportsTask(candidates []pickerNode, task string) bool {
	for _, candidate := range candidates {
		if capabilitiesSupportTask(candidate.capabilities, task) {
			return true
		}
	}
	return false
}

func capabilitiesSupportTask(capabilities []*pb.Capability, task string) bool {
	for _, capability := range capabilities {
		for _, ioTask := range capability.GetTasks() {
			if ioTask.GetName() == task {
				return true
			}
		}
	}
	return false
}

func taskNamesFromCapabilities(capabilities []*pb.Capability) []string {
	seen := make(map[string]struct{})
	var tasks []string
	for _, capability := range capabilities {
		for _, ioTask := range capability.GetTasks() {
			task := ioTask.GetName()
			if task == "" {
				continue
			}
			if _, ok := seen[task]; ok {
				continue
			}
			seen[task] = struct{}{}
			tasks = append(tasks, task)
		}
	}
	return tasks
}
