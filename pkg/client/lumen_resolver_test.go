package client

import (
	"testing"

	"github.com/edwinzhancn/lumen-sdk/pkg/discovery"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/serviceconfig"
)

// fakeResolverCC records the latest address set pushed by the resolver.
type fakeResolverCC struct {
	states []resolver.State
}

func (c *fakeResolverCC) UpdateState(state resolver.State) error {
	c.states = append(c.states, state)
	return nil
}

func (c *fakeResolverCC) ReportError(error) {}

func (c *fakeResolverCC) ParseServiceConfig(jsonSC string) *serviceconfig.ParseResult {
	return nil
}

func (c *fakeResolverCC) NewAddress(addresses []resolver.Address) {}

func nodeEvent(evType discovery.NodeEventType, id string, explicit bool) discovery.NodeEvent {
	return discovery.NodeEvent{
		Type:     evType,
		Identity: discovery.NewNodeIdentity("local", id),
		Resolved: discovery.ResolvedNode{
			Identity:  discovery.NewNodeIdentity("local", id),
			Addresses: []string{"192.168.1.10"},
			Port:      5866,
			Txt:       map[string]string{"v": "0.1.1"},
		},
		ExplicitRemove: explicit,
	}
}

func addressesOf(state resolver.State) map[string]bool {
	out := make(map[string]bool, len(state.Addresses))
	for _, addr := range state.Addresses {
		out[addr.Addr] = true
	}
	return out
}

func TestLumenResolverDiscoveryExpireReonline(t *testing.T) {
	cc := &fakeResolverCC{}
	r := &lumenResolver{
		cc:     cc,
		cancel: func() {},
		nodes:  make(map[string]resolvedEntry),
	}

	// 1. Node appears on the LAN: its address enters the state.
	r.handleEvent(nodeEvent(discovery.NodeDiscovered, "node-1", false))
	last := cc.states[len(cc.states)-1]
	if !addressesOf(last)["192.168.1.10:5866"] {
		t.Fatalf("discovered node address missing from state: %+v", last.Addresses)
	}

	// 2. Node goes offline (mDNS expiry after missed polls): address is removed.
	r.handleEvent(nodeEvent(discovery.NodeExpired, "node-1", false))
	last = cc.states[len(cc.states)-1]
	if len(last.Addresses) != 0 {
		t.Fatalf("expired node address should be removed, got %+v", last.Addresses)
	}

	// 3. Node comes back online: address is re-added.
	r.handleEvent(nodeEvent(discovery.NodeDiscovered, "node-1", false))
	last = cc.states[len(cc.states)-1]
	if !addressesOf(last)["192.168.1.10:5866"] {
		t.Fatalf("re-online node address missing from state: %+v", last.Addresses)
	}
}

func TestLumenResolverBrokerRemoveIsAlsoApplied(t *testing.T) {
	cc := &fakeResolverCC{}
	r := &lumenResolver{
		cc:     cc,
		cancel: func() {},
		nodes:  make(map[string]resolvedEntry),
	}

	r.handleEvent(nodeEvent(discovery.NodeDiscovered, "node-1", false))
	r.handleEvent(nodeEvent(discovery.NodeExpired, "node-1", true))
	last := cc.states[len(cc.states)-1]
	if len(last.Addresses) != 0 {
		t.Fatalf("broker removed node should vanish from state, got %+v", last.Addresses)
	}
}

func TestLumenResolverAttributesCarryProtocolCompatibility(t *testing.T) {
	cc := &fakeResolverCC{}
	r := &lumenResolver{
		cc:     cc,
		cancel: func() {},
		nodes:  make(map[string]resolvedEntry),
	}

	compatible := nodeEvent(discovery.NodeDiscovered, "node-ok", false)
	compatible.Resolved.Txt = map[string]string{"proto": "1.0"}
	incompatible := nodeEvent(discovery.NodeDiscovered, "node-old", false)
	incompatible.Resolved.Txt = map[string]string{"proto": "2.0"}
	r.handleEvent(compatible)
	r.handleEvent(incompatible)

	last := cc.states[len(cc.states)-1]
	if len(last.Addresses) != 2 {
		t.Fatalf("both nodes must be pushed to the balancer, got %+v", last.Addresses)
	}
	byAttr := make(map[bool]int)
	for _, addr := range last.Addresses {
		attr, ok := getNodeAttr(addr)
		if !ok {
			t.Fatal("address missing node attributes")
		}
		if attr.Compatible {
			byAttr[true]++
		} else {
			byAttr[false]++
			if attr.IncompatReason == "" {
				t.Fatalf("incompatible node must carry a reason: %+v", attr)
			}
		}
	}
	if byAttr[true] != 1 || byAttr[false] != 1 {
		t.Fatalf("expected one compatible and one incompatible address, got %+v", byAttr)
	}
}

func TestLumenResolverAttributesLegacyNodeWithoutHintIsOptimistic(t *testing.T) {
	cc := &fakeResolverCC{}
	r := &lumenResolver{
		cc:     cc,
		cancel: func() {},
		nodes:  make(map[string]resolvedEntry),
	}

	legacy := nodeEvent(discovery.NodeDiscovered, "node-legacy", false)
	legacy.Resolved.Txt = map[string]string{"v": "0.1.1"} // no proto key
	r.handleEvent(legacy)

	last := cc.states[len(cc.states)-1]
	for _, addr := range last.Addresses {
		attr, ok := getNodeAttr(addr)
		if !ok || !attr.Compatible {
			t.Fatalf("legacy node without protocol hint must stay optimistic: %+v", attr)
		}
	}
}
