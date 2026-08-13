package discovery

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type namedFakeResolver struct {
	name string

	mu      sync.Mutex
	watches int
	streams []chan NodeEvent
	errors  []error
	called  chan int
}

func (f *namedFakeResolver) SourceName() string { return f.name }

func (f *namedFakeResolver) Watch(context.Context) (<-chan NodeEvent, error) {
	f.mu.Lock()
	index := f.watches
	f.watches++
	var stream chan NodeEvent
	if index < len(f.streams) {
		stream = f.streams[index]
	}
	var err error
	if index < len(f.errors) {
		err = f.errors[index]
	}
	f.mu.Unlock()
	if f.called != nil {
		select {
		case f.called <- index + 1:
		default:
		}
	}
	if err != nil {
		return nil, err
	}
	if stream == nil {
		stream = make(chan NodeEvent)
	}
	return stream, nil
}

func TestCompositeResolverMergesBackends(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	a := NewStaticResolver([]string{"10.0.0.1:50051"}, "", nil)
	b := NewStaticResolver([]string{"10.0.0.2:50051"}, "", nil)
	resolver := NewCompositeResolver(a, b)
	merged, err := resolver.Watch(ctx)
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}

	events := collectEvents(t, merged, 2)
	seen := map[string]bool{}
	for _, event := range events {
		seen[event.Resolved.Endpoint()] = true
	}
	if !seen["10.0.0.1:50051"] || !seen["10.0.0.2:50051"] {
		t.Fatalf("merged events missing endpoints: %v", seen)
	}
	statuses := Statuses(resolver)
	if len(statuses) != 2 || statuses[0].State != BackendHealthy || statuses[1].State != BackendHealthy {
		t.Fatalf("unexpected source statuses: %+v", statuses)
	}

	cancel()
	awaitMDNSClose(t, merged)
}

func TestCompositeResolverRestartsOnlyClosedChildAndPreservesOtherSources(t *testing.T) {
	first := make(chan NodeEvent, 1)
	second := make(chan NodeEvent, 1)
	flaky := &namedFakeResolver{
		name:    "flaky",
		streams: []chan NodeEvent{first, second},
		called:  make(chan int, 4),
	}
	stable := NewStaticResolver([]string{"10.0.0.9:50051"}, "stable", nil)

	ctx, cancel := context.WithCancel(context.Background())
	resolver := NewCompositeResolver(flaky, stable)
	merged, _ := resolver.Watch(ctx)
	awaitWatchCall(t, flaky.called, 1)
	first <- discoveredEvent("local", "flaky-node", "10.0.0.8", 50051)

	events := collectEvents(t, merged, 2)
	seen := map[string]bool{}
	for _, event := range events {
		seen[event.Resolved.Key()] = true
	}
	if !seen["stable-static-10.0.0.9:50051"] || !seen["local-flaky-node"] {
		t.Fatalf("initial merged state missing node: %v", seen)
	}

	close(first)
	awaitWatchCall(t, flaky.called, 2)
	second <- discoveredEvent("local", "late-node", "10.0.0.7", 50051)
	late := awaitEvent(t, merged, time.Second)
	if late.Type != NodeDiscovered || late.Resolved.Key() != "local-late-node" {
		t.Fatalf("late event = %+v", late)
	}
	select {
	case event := <-merged:
		if event.Type == NodeExpired && event.Resolved.Key() == "stable-static-10.0.0.9:50051" {
			t.Fatalf("stable source was revoked by flaky source restart: %+v", event)
		}
	case <-time.After(25 * time.Millisecond):
	}

	cancel()
	close(second)
	awaitMDNSClose(t, merged)
}

func TestCompositeResolverRetriesWatchStartFailure(t *testing.T) {
	stream := make(chan NodeEvent, 1)
	flaky := &namedFakeResolver{
		name:    "flaky",
		streams: []chan NodeEvent{nil, stream},
		errors:  []error{errors.New("boom"), nil},
		called:  make(chan int, 4),
	}
	ctx, cancel := context.WithCancel(context.Background())
	resolver := NewCompositeResolver(flaky)
	merged, err := resolver.Watch(ctx)
	if err != nil {
		t.Fatalf("composite must start in degraded mode: %v", err)
	}
	awaitWatchCall(t, flaky.called, 1)
	awaitWatchCall(t, flaky.called, 2)
	stream <- discoveredEvent("local", "recovered", "10.0.0.6", 50051)
	event := awaitEvent(t, merged, time.Second)
	if event.Resolved.Key() != "local-recovered" {
		t.Fatalf("recovered event = %+v", event)
	}
	cancel()
	close(stream)
	awaitMDNSClose(t, merged)
}

func TestCompositeResolverReconcilesIdentityOwnershipAcrossSources(t *testing.T) {
	aStream := make(chan NodeEvent, 4)
	bStream := make(chan NodeEvent, 4)
	a := &namedFakeResolver{name: "mdns", streams: []chan NodeEvent{aStream}}
	b := &namedFakeResolver{name: "broker", streams: []chan NodeEvent{bStream}}
	ctx, cancel := context.WithCancel(context.Background())
	merged, _ := NewCompositeResolver(a, b).Watch(ctx)

	aEvent := discoveredEvent("lab", "node-1", "10.0.0.1", 5866)
	bEvent := discoveredEvent("lab", "node-1", "10.0.0.2", 5866)
	aStream <- aEvent
	first := awaitEvent(t, merged, time.Second)
	if first.Resolved.Source != "mdns" {
		t.Fatalf("primary source = %q, want mdns", first.Resolved.Source)
	}
	bStream <- bEvent
	ownedByBoth := awaitEvent(t, merged, time.Second)
	if len(ownedByBoth.Resolved.Sources) != 2 || ownedByBoth.Resolved.Endpoint() != "10.0.0.1:5866" {
		t.Fatalf("ownership merge = %+v", ownedByBoth.Resolved)
	}

	aStream <- NodeEvent{Type: NodeExpired, Resolved: aEvent.Resolved}
	failover := awaitEvent(t, merged, time.Second)
	if failover.Type != NodeDiscovered || failover.Resolved.Source != "broker" || failover.Resolved.Endpoint() != "10.0.0.2:5866" {
		t.Fatalf("source failover = %+v", failover)
	}
	bStream <- NodeEvent{Type: NodeExpired, Resolved: bEvent.Resolved}
	expired := awaitEvent(t, merged, time.Second)
	if expired.Type != NodeExpired {
		t.Fatalf("final event = %+v, want expired", expired)
	}

	cancel()
	close(aStream)
	close(bStream)
	awaitMDNSClose(t, merged)
}

func discoveredEvent(deployment, node, address string, port int) NodeEvent {
	return NodeEvent{
		Type: NodeDiscovered,
		Resolved: ResolvedNode{
			Identity:     NewNodeIdentity(deployment, node),
			Addresses:    []string{address},
			Port:         port,
			LastObserved: time.Now().UTC(),
		},
	}
}

func awaitWatchCall(t *testing.T, called <-chan int, want int) {
	t.Helper()
	select {
	case got := <-called:
		if got != want {
			t.Fatalf("watch call = %d, want %d", got, want)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for watch call %d", want)
	}
}
