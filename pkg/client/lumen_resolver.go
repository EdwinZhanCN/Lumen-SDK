package client

import (
	"context"
	"strings"
	"sync"

	"github.com/edwinzhancn/lumen-sdk/pkg/discovery"
	"go.uber.org/zap"
	"google.golang.org/grpc/attributes"
	"google.golang.org/grpc/resolver"
)

const lumenScheme = "lumen"

// nodeAttrKey is the attributes key for node metadata attached to each address.
type nodeAttrKey struct{}

// nodeAttr carries discovery metadata for a single resolved address.
type nodeAttr struct {
	Identity discovery.NodeIdentity
	Tasks    []string
	Txt      map[string]string
}

func setNodeAttr(addr resolver.Address, attr nodeAttr) resolver.Address {
	addr.BalancerAttributes = attributes.New(nodeAttrKey{}, attr)
	return addr
}

func getNodeAttr(addr resolver.Address) (nodeAttr, bool) {
	if addr.BalancerAttributes == nil {
		return nodeAttr{}, false
	}
	v := addr.BalancerAttributes.Value(nodeAttrKey{})
	if v == nil {
		return nodeAttr{}, false
	}
	attr, ok := v.(nodeAttr)
	return attr, ok
}

// lumenResolverBuilder implements resolver.Builder. It bridges NodeResolver
// events into gRPC's address resolution framework.
type lumenResolverBuilder struct {
	nodeResolver discovery.NodeResolver
	logger       *zap.Logger
}

func (b *lumenResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, _ resolver.BuildOptions) (resolver.Resolver, error) {
	ctx, cancel := context.WithCancel(context.Background())
	r := &lumenResolver{
		cc:     cc,
		cancel: cancel,
		nodes:  make(map[string]resolvedEntry),
		logger: b.logger,
	}
	go r.watch(ctx, b.nodeResolver)
	return r, nil
}

func (b *lumenResolverBuilder) Scheme() string { return lumenScheme }

type resolvedEntry struct {
	node      discovery.ResolvedNode
	endpoints []string
}

// lumenResolver watches discovery events and pushes address updates to gRPC.
type lumenResolver struct {
	cc     resolver.ClientConn
	cancel context.CancelFunc
	mu     sync.Mutex
	nodes  map[string]resolvedEntry
	logger *zap.Logger
}

func (r *lumenResolver) watch(ctx context.Context, nr discovery.NodeResolver) {
	ch, err := nr.Watch(ctx)
	if err != nil {
		r.logger.Error("resolver watch failed", zap.Error(err))
		return
	}

	for ev := range ch {
		r.handleEvent(ev)
	}
}

func (r *lumenResolver) handleEvent(ev discovery.NodeEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()

	switch ev.Type {
	case discovery.NodeDiscovered:
		resolved := resolvedFromEvent(ev)
		if resolved.Identity.IsZero() {
			return
		}
		key := resolved.Key()
		endpoints := resolved.CandidateEndpoints()
		r.nodes[key] = resolvedEntry{node: resolved, endpoints: endpoints}

	case discovery.NodeExpired:
		resolved := resolvedFromEvent(ev)
		key := eventKey(ev, resolved)
		if key == "" {
			return
		}
		if ev.ExplicitRemove {
			delete(r.nodes, key)
		}

	case discovery.NodeResolveFailed:
		// Don't remove — the balancer handles degraded state.
	}

	r.pushStateLocked()
}

func (r *lumenResolver) pushStateLocked() {
	var addrs []resolver.Address
	for _, entry := range r.nodes {
		if len(entry.endpoints) == 0 {
			continue
		}
		attr := nodeAttr{
			Identity: entry.node.Identity,
			Tasks:    entry.node.HintTasks(),
			Txt:      entry.node.Txt,
		}
		// Use first endpoint as primary address; the balancer creates one
		// SubConn per unique address.
		addr := setNodeAttr(resolver.Address{Addr: entry.endpoints[0]}, attr)
		addrs = append(addrs, addr)
	}

	r.cc.UpdateState(resolver.State{Addresses: addrs})
}

func (r *lumenResolver) ResolveNow(_ resolver.ResolveNowOptions) {}

func (r *lumenResolver) Close() {
	r.cancel()
}

// resolvedFromEvent reconstructs a ResolvedNode from a NodeEvent.
func resolvedFromEvent(ev discovery.NodeEvent) discovery.ResolvedNode {
	resolved := ev.Resolved
	if resolved.Identity.IsZero() && ev.Identity.NodeID != "" {
		resolved.Identity = ev.Identity
	}
	if len(resolved.Addresses) == 0 && len(ev.Addresses) > 0 {
		resolved.Addresses = append([]string(nil), ev.Addresses...)
	}
	if resolved.Txt == nil && ev.Txt != nil {
		resolved.Txt = copyStringMap(ev.Txt)
	}
	if resolved.Txt == nil {
		resolved.Txt = map[string]string{}
	}
	if len(ev.Tasks) > 0 && resolved.Txt["tasks"] == "" {
		resolved.Txt["tasks"] = strings.Join(ev.Tasks, ",")
	}
	return resolved.Normalized()
}

// eventKey extracts the node key from the resolved node or event identity.
func eventKey(ev discovery.NodeEvent, resolved discovery.ResolvedNode) string {
	if !resolved.Identity.IsZero() {
		return resolved.Key()
	}
	if ev.Identity.NodeID != "" {
		return ev.Identity.Key()
	}
	return ""
}
