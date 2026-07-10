package discovery

import (
	"context"
	"testing"
	"time"
)

func collectEvents(t *testing.T, ch <-chan NodeEvent, want int) []NodeEvent {
	t.Helper()
	events := make([]NodeEvent, 0, want)
	timeout := time.After(2 * time.Second)
	for len(events) < want {
		select {
		case ev, ok := <-ch:
			if !ok {
				t.Fatalf("event channel closed after %d events, want %d", len(events), want)
			}
			events = append(events, ev)
		case <-timeout:
			t.Fatalf("timed out after %d events, want %d", len(events), want)
		}
	}
	return events
}

func TestStaticResolverEmitsConfiguredEndpoints(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r := NewStaticResolver([]string{"10.0.0.5:50051", " nas.local:50052 ", "bogus", ""}, "", nil)
	ch, err := r.Watch(ctx)
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}

	events := collectEvents(t, ch, 2)

	first := events[0]
	if first.Type != NodeDiscovered {
		t.Fatalf("event type = %v, want NodeDiscovered", first.Type)
	}
	if first.Identity.DeploymentID != DefaultDeploymentID {
		t.Fatalf("deployment = %q, want default", first.Identity.DeploymentID)
	}
	if len(first.Addresses) != 1 || first.Addresses[0] != "10.0.0.5:50051" {
		t.Fatalf("addresses = %v, want [10.0.0.5:50051]", first.Addresses)
	}
	if events[1].Addresses[0] != "nas.local:50052" {
		t.Fatalf("second endpoint = %v, want trimmed nas.local:50052", events[1].Addresses)
	}
	if events[0].Identity.Key() == events[1].Identity.Key() {
		t.Fatal("static endpoints must have distinct identities")
	}

	// Channel stays open (no expiry) until cancellation, then closes.
	select {
	case ev, ok := <-ch:
		if ok {
			t.Fatalf("unexpected extra event: %+v", ev)
		}
		t.Fatal("channel closed before ctx cancellation")
	case <-time.After(50 * time.Millisecond):
	}
	cancel()
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected channel close after cancel, got event")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("channel did not close after ctx cancellation")
	}
}

// CompositeResolver behavior is covered in composite_resolver_test.go.
