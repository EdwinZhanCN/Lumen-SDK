package discovery

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

const (
	resolverRestartBackoffMin = 100 * time.Millisecond
	resolverRestartBackoffMax = 5 * time.Second
)

// CompositeResolver supervises each discovery source independently and
// reconciles source ownership before publishing the aggregate event stream.
// One failed or unexpectedly closed child cannot terminate healthy sources.
type CompositeResolver struct {
	children []compositeChild
	logger   *zap.Logger

	statusMu sync.RWMutex
	status   []compositeSupervisorStatus
}

type compositeChild struct {
	resolver NodeResolver
	name     string
}

type compositeSupervisorStatus struct {
	status   ResolverStatus
	degraded bool
}

// NewCompositeResolver combines the given backends. Nil entries are dropped.
// Even a single child is wrapped so startup failure and unexpected watch
// closure receive the same supervision contract as multi-source clients.
func NewCompositeResolver(resolvers ...NodeResolver) NodeResolver {
	return NewCompositeResolverWithLogger(nil, resolvers...)
}

// NewCompositeResolverWithLogger is the diagnostic-aware constructor used by
// LumenClient. NewCompositeResolver remains available for source compatibility.
func NewCompositeResolverWithLogger(logger *zap.Logger, resolvers ...NodeResolver) NodeResolver {
	children := make([]compositeChild, 0, len(resolvers))
	nameCounts := make(map[string]int)
	for index, resolver := range resolvers {
		if resolver == nil {
			continue
		}
		name := resolverSourceName(resolver, index)
		nameCounts[name]++
		if nameCounts[name] > 1 {
			name = fmt.Sprintf("%s_%d", name, nameCounts[name])
		}
		children = append(children, compositeChild{resolver: resolver, name: name})
	}
	statuses := make([]compositeSupervisorStatus, len(children))
	for index, child := range children {
		statuses[index].status = ResolverStatus{Source: child.name, State: BackendStarting}
	}
	return &CompositeResolver{
		children: children,
		logger:   ensureLogger(logger),
		status:   statuses,
	}
}

func (c *CompositeResolver) SourceName() string { return "composite" }

func (c *CompositeResolver) Watch(ctx context.Context) (<-chan NodeEvent, error) {
	out := make(chan NodeEvent, 32)
	go c.watch(ctx, out)
	return out, nil
}

type compositeDelivery struct {
	sourceIndex int
	event       NodeEvent
}

func (c *CompositeResolver) watch(ctx context.Context, out chan<- NodeEvent) {
	defer close(out)
	emitter := newSnapshotEmitter()
	publisherDone := make(chan struct{})
	go func() {
		defer close(publisherDone)
		emitter.run(ctx, out)
	}()

	deliveries := make(chan compositeDelivery, 128)
	var supervisors sync.WaitGroup
	for index := range c.children {
		supervisors.Add(1)
		go func(index int) {
			defer supervisors.Done()
			c.supervise(ctx, index, deliveries)
		}(index)
	}

	owned := make([]map[string]ResolvedNode, len(c.children))
	for index := range owned {
		owned[index] = make(map[string]ResolvedNode)
	}
	for {
		select {
		case <-ctx.Done():
			supervisors.Wait()
			<-publisherDone
			return
		case delivery := <-deliveries:
			if delivery.sourceIndex < 0 || delivery.sourceIndex >= len(owned) {
				continue
			}
			event := delivery.event
			resolved := event.Resolved.Normalized()
			if resolved.Identity.IsZero() {
				continue
			}
			source := c.children[delivery.sourceIndex].name
			resolved.Source = source
			resolved.Sources = []string{source}
			switch event.Type {
			case NodeDiscovered:
				owned[delivery.sourceIndex][resolved.Key()] = resolved.Normalized()
			case NodeExpired:
				delete(owned[delivery.sourceIndex], resolved.Key())
			case NodeResolveFailed:
				continue
			}
			emitter.replace(mergeOwnedSnapshots(c.children, owned))
		}
	}
}

func (c *CompositeResolver) supervise(ctx context.Context, index int, deliveries chan<- compositeDelivery) {
	child := c.children[index]
	backoff := resolverRestartBackoffMin
	restartCount := 0
	for {
		if ctx.Err() != nil {
			return
		}
		c.markSupervisorStarting(index)
		stream, err := child.resolver.Watch(ctx)
		if err != nil {
			c.markSupervisorFailure(index, ErrorCodeWatchStartFailed)
			c.logger.Warn("discovery backend failed to start; restarting",
				zap.String("source", child.name),
				zap.String("error_code", ErrorCodeWatchStartFailed),
				zap.Duration("backoff", backoff),
				zap.Error(err),
			)
			if !waitForRestart(ctx, backoff) {
				return
			}
			restartCount++
			backoff = nextResolverBackoff(backoff)
			continue
		}

		c.markSupervisorActive(index)
		c.logger.Info("discovery backend watch started",
			zap.String("source", child.name),
			zap.Int("restart_count", restartCount),
		)
		backoff = resolverRestartBackoffMin
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-stream:
				if !ok {
					c.markSupervisorFailure(index, ErrorCodeWatchClosed)
					c.logger.Warn("discovery backend watch closed unexpectedly; restarting",
						zap.String("source", child.name),
						zap.String("error_code", ErrorCodeWatchClosed),
						zap.Duration("backoff", backoff),
					)
					if !waitForRestart(ctx, backoff) {
						return
					}
					restartCount++
					backoff = nextResolverBackoff(backoff)
					goto restart
				}
				select {
				case deliveries <- compositeDelivery{sourceIndex: index, event: event}:
				case <-ctx.Done():
					return
				}
			}
		}
	restart:
	}
}

func (c *CompositeResolver) ResolveNow() {
	for _, child := range c.children {
		RequestResolveNow(child.resolver)
	}
}

func (c *CompositeResolver) ResolverStatuses() []ResolverStatus {
	c.statusMu.RLock()
	supervisorStatuses := append([]compositeSupervisorStatus(nil), c.status...)
	c.statusMu.RUnlock()

	out := make([]ResolverStatus, 0, len(c.children))
	for index, child := range c.children {
		supervisor := supervisorStatuses[index]
		childStatuses := Statuses(child.resolver)
		status := supervisor.status
		if len(childStatuses) > 0 {
			status = childStatuses[0]
			status.Source = child.name
			if supervisor.degraded {
				status.State = BackendDegraded
				status.LastErrorCode = supervisor.status.LastErrorCode
				status.ConsecutiveFailures = supervisor.status.ConsecutiveFailures
			}
		}
		out = append(out, status)
	}
	return out
}

func (c *CompositeResolver) markSupervisorStarting(index int) {
	c.statusMu.Lock()
	defer c.statusMu.Unlock()
	if index < 0 || index >= len(c.status) {
		return
	}
	if c.status[index].status.LastScanCompletedAt.IsZero() {
		c.status[index].status.State = BackendStarting
	}
}

func (c *CompositeResolver) markSupervisorActive(index int) {
	c.statusMu.Lock()
	defer c.statusMu.Unlock()
	if index < 0 || index >= len(c.status) {
		return
	}
	c.status[index].degraded = false
	c.status[index].status.State = BackendHealthy
	c.status[index].status.ConsecutiveFailures = 0
	c.status[index].status.LastErrorCode = ErrorCodeNone
}

func (c *CompositeResolver) markSupervisorFailure(index int, code string) {
	c.statusMu.Lock()
	defer c.statusMu.Unlock()
	if index < 0 || index >= len(c.status) {
		return
	}
	status := &c.status[index]
	status.degraded = true
	status.status.State = BackendDegraded
	status.status.ConsecutiveFailures++
	status.status.LastErrorCode = code
	status.status.LastOutcome = ScanOutcomeFailed
	status.status.LastScanCompletedAt = time.Now().UTC()
}

func mergeOwnedSnapshots(children []compositeChild, owned []map[string]ResolvedNode) map[string]ResolvedNode {
	identities := make(map[string]struct{})
	for _, snapshot := range owned {
		for key := range snapshot {
			identities[key] = struct{}{}
		}
	}
	merged := make(map[string]ResolvedNode, len(identities))
	for key := range identities {
		var selected ResolvedNode
		var sources []string
		for index, snapshot := range owned {
			node, ok := snapshot[key]
			if !ok {
				continue
			}
			sources = append(sources, children[index].name)
			if selected.Identity.IsZero() {
				selected = node
				continue
			}
			if node.LastObserved.After(selected.LastObserved) {
				selected.LastObserved = node.LastObserved
			}
		}
		if selected.Identity.IsZero() {
			continue
		}
		selected.Source = sources[0]
		selected.Sources = append([]string(nil), sources...)
		merged[key] = selected.Normalized()
	}
	return merged
}

func resolverSourceName(resolver NodeResolver, index int) string {
	if named, ok := resolver.(SourceNameProvider); ok {
		if name := boundedSourceName(named.SourceName()); name != "" {
			return name
		}
	}
	return fmt.Sprintf("resolver_%d", index+1)
}

func boundedSourceName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if len(name) > 32 {
		name = name[:32]
	}
	var out strings.Builder
	for _, char := range name {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '_' || char == '-' {
			out.WriteRune(char)
		}
	}
	return out.String()
}

func waitForRestart(ctx context.Context, backoff time.Duration) bool {
	timer := time.NewTimer(backoff)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func nextResolverBackoff(current time.Duration) time.Duration {
	current *= 2
	if current > resolverRestartBackoffMax {
		return resolverRestartBackoffMax
	}
	return current
}
