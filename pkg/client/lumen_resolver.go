package client

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/edwinzhancn/lumen-sdk/pkg/discovery"
	"go.uber.org/zap"
	"google.golang.org/grpc/attributes"
	"google.golang.org/grpc/resolver"
)

const lumenScheme = "lumen"

type nodeAttrKey struct{}

type nodeAttr struct {
	Identity     discovery.NodeIdentity
	Metadata     map[string]string
	Sources      []string
	LastObserved time.Time
}

func setNodeAttr(addr resolver.Address, attr nodeAttr) resolver.Address {
	addr.BalancerAttributes = attributes.New(nodeAttrKey{}, attr)
	return addr
}

func getNodeAttr(addr resolver.Address) (nodeAttr, bool) {
	if addr.BalancerAttributes == nil {
		return nodeAttr{}, false
	}
	attr, ok := addr.BalancerAttributes.Value(nodeAttrKey{}).(nodeAttr)
	return attr, ok
}

type lumenResolverBuilder struct {
	nodeResolver discovery.NodeResolver
	parentCtx    context.Context
	logger       *zap.Logger
}

func (b *lumenResolverBuilder) Build(_ resolver.Target, cc resolver.ClientConn, _ resolver.BuildOptions) (resolver.Resolver, error) {
	parentCtx := b.parentCtx
	if parentCtx == nil {
		parentCtx = context.Background()
	}
	ctx, cancel := context.WithCancel(parentCtx)
	r := &lumenResolver{
		cc:           cc,
		cancel:       cancel,
		nodeResolver: b.nodeResolver,
		nodes:        make(map[string]discovery.ResolvedNode),
		logger:       b.logger,
	}
	go r.watch(ctx, b.nodeResolver)
	return r, nil
}

func (b *lumenResolverBuilder) Scheme() string { return lumenScheme }

type lumenResolver struct {
	cc           resolver.ClientConn
	cancel       context.CancelFunc
	nodeResolver discovery.NodeResolver
	mu           sync.Mutex
	nodes        map[string]discovery.ResolvedNode
	logger       *zap.Logger
}

func (r *lumenResolver) watch(ctx context.Context, nr discovery.NodeResolver) {
	ch, err := nr.Watch(ctx)
	if err != nil {
		r.logger.Error("resolver watch failed", zap.Error(err))
		return
	}
	for event := range ch {
		r.handleEvent(event)
	}
}

func (r *lumenResolver) handleEvent(event discovery.NodeEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()

	resolved := event.Resolved.Normalized()
	switch event.Type {
	case discovery.NodeDiscovered:
		if resolved.Identity.IsZero() {
			return
		}
		r.nodes[resolved.Key()] = resolved
	case discovery.NodeExpired:
		if !resolved.Identity.IsZero() {
			delete(r.nodes, resolved.Key())
		}
	case discovery.NodeResolveFailed:
		// Resolution failures do not make liveness or compatibility claims.
		return
	}
	r.pushStateLocked()
}

func (r *lumenResolver) pushStateLocked() {
	keys := make([]string, 0, len(r.nodes))
	for key := range r.nodes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	addrs := make([]resolver.Address, 0, len(keys))
	for _, key := range keys {
		node := r.nodes[key]
		endpoints := node.CandidateEndpoints()
		if len(endpoints) == 0 {
			continue
		}
		attr := nodeAttr{
			Identity:     node.Identity,
			Metadata:     discoveryMetadata(node),
			Sources:      append([]string(nil), node.Sources...),
			LastObserved: node.LastObserved,
		}
		addrs = append(addrs, setNodeAttr(resolver.Address{Addr: endpoints[0]}, attr))
	}
	r.cc.UpdateState(resolver.State{Addresses: addrs})
}

func discoveryMetadata(node discovery.ResolvedNode) map[string]string {
	metadata := make(map[string]string, 2)
	if version := node.Version(); version != "" {
		metadata["v"] = version
	}
	if runtime := node.Runtime(); runtime != "" {
		metadata["runtime"] = runtime
	}
	return metadata
}

func (r *lumenResolver) ResolveNow(_ resolver.ResolveNowOptions) {
	discovery.RequestResolveNow(r.nodeResolver)
}

func (r *lumenResolver) Close() { r.cancel() }
