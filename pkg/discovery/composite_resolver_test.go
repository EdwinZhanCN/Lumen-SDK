package discovery

import (
	"context"
	"errors"
	"testing"
	"time"
)

// fakeResolver is a minimal, directly-controllable NodeResolver test double,
// used where StaticResolver's real address-parsing behavior isn't the point.
type fakeResolver struct {
	ch  chan NodeEvent
	err error
}

func (f *fakeResolver) Watch(ctx context.Context) (<-chan NodeEvent, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.ch, nil
}

// TestCompositeResolverMergesBackends and TestCompositeResolverSingleBackendPassthrough
// were relocated from static_resolver_test.go: they exercise CompositeResolver
// behavior using StaticResolver only as a convenient fake backend.

func TestCompositeResolverMergesBackends(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	a := NewStaticResolver([]string{"10.0.0.1:50051"}, "", nil)
	b := NewStaticResolver([]string{"10.0.0.2:50051"}, "", nil)
	merged, err := NewCompositeResolver(a, b, nil).Watch(ctx)
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}

	events := collectEvents(t, merged, 2)
	seen := map[string]bool{}
	for _, ev := range events {
		seen[ev.Addresses[0]] = true
	}
	if !seen["10.0.0.1:50051"] || !seen["10.0.0.2:50051"] {
		t.Fatalf("merged events missing endpoints: %v", seen)
	}

	cancel()
	deadline := time.After(2 * time.Second)
	for {
		select {
		case _, ok := <-merged:
			if !ok {
				return // closed once all backends stopped
			}
		case <-deadline:
			t.Fatal("merged channel did not close after ctx cancellation")
		}
	}
}

func TestCompositeResolverSingleBackendPassthrough(t *testing.T) {
	r := NewStaticResolver([]string{"10.0.0.1:50051"}, "", nil)
	if got := NewCompositeResolver(nil, r); got != NodeResolver(r) {
		t.Fatalf("single backend should be returned as-is, got %T", got)
	}
}

// TestCompositeResolverPropagatesBackendWatchError characterizes the current
// fail-fast fan-out: if any backend's Watch call fails to start, the whole
// composite fails to start rather than degrading to the remaining backends.
func TestCompositeResolverPropagatesBackendWatchError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	good := NewStaticResolver([]string{"10.0.0.1:50051"}, "", nil)
	bad := &fakeResolver{err: errors.New("boom")}

	if _, err := NewCompositeResolver(good, bad).Watch(ctx); err == nil {
		t.Fatal("expected an error when one backend fails to start, got nil")
	}
}

// TestCompositeResolverStaysOpenUntilAllBackendsClose guards against a
// refactor accidentally closing the merged channel as soon as the first
// backend finishes, which would silently drop events from slower backends
// (e.g. mDNS) still in flight.
func TestCompositeResolverStaysOpenUntilAllBackendsClose(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fast := &fakeResolver{ch: make(chan NodeEvent)}
	close(fast.ch) // this backend has nothing to say and closes immediately

	slow := NewStaticResolver([]string{"10.0.0.9:50051"}, "", nil) // open until ctx cancel

	merged, err := NewCompositeResolver(fast, slow).Watch(ctx)
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}

	ev := awaitEvent(t, merged, 2*time.Second)
	if len(ev.Addresses) != 1 || ev.Addresses[0] != "10.0.0.9:50051" {
		t.Fatalf("addresses = %v, want [10.0.0.9:50051]", ev.Addresses)
	}

	select {
	case _, ok := <-merged:
		if ok {
			t.Fatal("unexpected extra event")
		}
		t.Fatal("merged channel closed before all backends stopped")
	case <-time.After(100 * time.Millisecond):
	}

	cancel()
	select {
	case _, ok := <-merged:
		if ok {
			t.Fatal("expected close, got event")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("merged channel did not close after ctx cancellation")
	}
}

// TestCompositeResolverDoesNotDeduplicateAcrossSources documents that
// deduplication across discovery sources (e.g. the same node visible via
// mDNS and a future Broker resolver) is explicitly not CompositeResolver's
// job; the pool/registry layer handles merging.
func TestCompositeResolverDoesNotDeduplicateAcrossSources(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	a := NewStaticResolver([]string{"10.0.0.5:50051"}, "dup", nil)
	b := NewStaticResolver([]string{"10.0.0.5:50051"}, "dup", nil)

	merged, err := NewCompositeResolver(a, b).Watch(ctx)
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}

	events := collectEvents(t, merged, 2)
	if events[0].Identity.Key() != events[1].Identity.Key() {
		t.Fatalf("expected both sources to report the same identity, got %q and %q",
			events[0].Identity.Key(), events[1].Identity.Key())
	}
}
