package discovery

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// awaitEvent reads exactly one event within timeout, or fails the test. It
// exists alongside collectEvents (static_resolver_test.go) because reconnect
// tests need a longer, test-specific deadline to comfortably clear the
// BrokerResolver's 1s minimum backoff.
func awaitEvent(t *testing.T, ch <-chan NodeEvent, timeout time.Duration) NodeEvent {
	t.Helper()
	select {
	case ev, ok := <-ch:
		if !ok {
			t.Fatal("event channel closed while waiting for event")
		}
		return ev
	case <-time.After(timeout):
		t.Fatal("timed out waiting for event")
		return NodeEvent{}
	}
}

func TestParseNodeEventsSnapshot(t *testing.T) {
	raw := []byte(`{
		"type": "snapshot",
		"nodes": [
			{"node_id": "node-a", "address": "10.0.0.1:50051", "tasks": ["ocr"], "txt": {"v": "1.0"}},
			{"node_id": "lab-node-b", "deployment_id": "lab", "addresses": ["10.0.0.2"], "port": 50052}
		]
	}`)

	events, err := parseNodeEvents(raw)
	if err != nil {
		t.Fatalf("parseNodeEvents: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("got %d events, want 2", len(events))
	}

	a := events[0]
	if a.Type != NodeDiscovered {
		t.Fatalf("event 0 type = %v, want NodeDiscovered", a.Type)
	}
	if a.Identity.Key() != "local-node-a" {
		t.Fatalf("event 0 identity = %q, want local-node-a", a.Identity.Key())
	}
	if len(a.Addresses) != 1 || a.Addresses[0] != "10.0.0.1:50051" {
		t.Fatalf("event 0 addresses = %v, want [10.0.0.1:50051]", a.Addresses)
	}
	if len(a.Tasks) != 1 || a.Tasks[0] != "ocr" {
		t.Fatalf("event 0 tasks = %v, want [ocr]", a.Tasks)
	}
	if a.Txt["v"] != "1.0" {
		t.Fatalf("event 0 txt = %v, want v=1.0", a.Txt)
	}

	// A per-node deployment_id overrides the resolver's default deployment.
	b := events[1]
	if b.Identity.Key() != "lab-node-b" {
		t.Fatalf("event 1 identity = %q, want lab-node-b", b.Identity.Key())
	}
	if len(b.Addresses) != 1 || b.Addresses[0] != "10.0.0.2:50052" {
		t.Fatalf("event 1 addresses = %v, want [10.0.0.2:50052]", b.Addresses)
	}
}

func TestParseNodeEventsAdded(t *testing.T) {
	raw := []byte(`{"type": "added", "node": {"node_id": "node-c", "address": "10.0.0.3:50051"}}`)

	events, err := parseNodeEvents(raw)
	if err != nil {
		t.Fatalf("parseNodeEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	ev := events[0]
	if ev.Type != NodeDiscovered {
		t.Fatalf("type = %v, want NodeDiscovered", ev.Type)
	}
	if ev.ExplicitRemove {
		t.Fatal("added event must not be ExplicitRemove")
	}
	if ev.Identity.Key() != "local-node-c" {
		t.Fatalf("identity = %q, want local-node-c", ev.Identity.Key())
	}
}

func TestParseNodeEventsRemoved(t *testing.T) {
	raw := []byte(`{"type": "removed", "node_id": "node-d"}`)

	events, err := parseNodeEvents(raw)
	if err != nil {
		t.Fatalf("parseNodeEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	ev := events[0]
	if ev.Type != NodeExpired {
		t.Fatalf("type = %v, want NodeExpired", ev.Type)
	}
	if !ev.ExplicitRemove {
		t.Fatal("Broker removed event must be ExplicitRemove (unlike mDNS TTL expiry)")
	}
	if ev.Identity.Key() != "local-node-d" {
		t.Fatalf("identity = %q, want local-node-d", ev.Identity.Key())
	}
}

func TestParseNodeEventsErrors(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{"malformed json", `{not valid json`},
		{"unknown type", `{"type": "unknown"}`},
		{"added missing node", `{"type": "added"}`},
		{"removed missing node_id", `{"type": "removed"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parseNodeEvents([]byte(tt.raw)); err == nil {
				t.Fatalf("parseNodeEvents(%q) = nil error, want error", tt.raw)
			}
		})
	}
}

func TestBrokerAddresses(t *testing.T) {
	tests := []struct {
		name      string
		node      brokerNode
		wantAddrs []string
		wantPort  int
	}{
		{
			name:      "address with port",
			node:      brokerNode{Address: "10.0.0.1:50051"},
			wantAddrs: []string{"10.0.0.1"},
			wantPort:  50051,
		},
		{
			name:      "explicit port wins over parsed port",
			node:      brokerNode{Address: "10.0.0.1:50051", Port: 9999},
			wantAddrs: []string{"10.0.0.1"},
			wantPort:  9999,
		},
		{
			name:      "addresses list combined with legacy address field",
			node:      brokerNode{Addresses: []string{"10.0.0.2"}, Address: "10.0.0.1:50051"},
			wantAddrs: []string{"10.0.0.2", "10.0.0.1"},
			wantPort:  50051,
		},
		{
			name:      "unparseable address falls back to raw string",
			node:      brokerNode{Address: "not-a-host-port", Port: 50051},
			wantAddrs: []string{"not-a-host-port"},
			wantPort:  50051,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAddrs, gotPort := brokerAddresses(tt.node)
			if len(gotAddrs) != len(tt.wantAddrs) {
				t.Fatalf("addresses = %v, want %v", gotAddrs, tt.wantAddrs)
			}
			for i := range tt.wantAddrs {
				if gotAddrs[i] != tt.wantAddrs[i] {
					t.Fatalf("addresses = %v, want %v", gotAddrs, tt.wantAddrs)
				}
			}
			if gotPort != tt.wantPort {
				t.Fatalf("port = %d, want %d", gotPort, tt.wantPort)
			}
		})
	}
}

func TestWsSchemeAndWsHost(t *testing.T) {
	tests := []struct {
		brokerURL  string
		wantScheme string
		wantHost   string
	}{
		{"http://localhost:5866", "ws", "localhost:5866"},
		{"https://broker.example.com", "wss", "broker.example.com"},
		{"localhost:5866", "ws", "localhost:5866"},
		// Documented quirk: wsScheme only checks the first 5 bytes, so any
		// string literally starting with "https" (not just an https:// URL)
		// is treated as secure. Locked down here so a future rename doesn't
		// silently "fix" this and change behavior for real deployments.
		{"httpsomething", "wss", "httpsomething"},
	}

	for _, tt := range tests {
		t.Run(tt.brokerURL, func(t *testing.T) {
			if got := wsScheme(tt.brokerURL); got != tt.wantScheme {
				t.Fatalf("wsScheme(%q) = %q, want %q", tt.brokerURL, got, tt.wantScheme)
			}
			if got := wsHost(tt.brokerURL); got != tt.wantHost {
				t.Fatalf("wsHost(%q) = %q, want %q", tt.brokerURL, got, tt.wantHost)
			}
		})
	}
}

// TestBrokerResolverReconnectsAfterDisconnectWithoutDuplicating drives a real
// WebSocket server that serves one snapshot, drops the connection, and serves
// a different snapshot on the next attempt. It characterizes BrokerResolver's
// reconnect behavior (Milestone 0 acceptance: reconnection must not create
// duplicate active nodes at the event-stream level).
func TestBrokerResolverReconnectsAfterDisconnectWithoutDuplicating(t *testing.T) {
	var attempts atomic.Int32
	upgrader := websocket.Upgrader{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		if n == 1 {
			_ = conn.WriteJSON(brokerNodeEvent{
				Type:  "snapshot",
				Nodes: []brokerNode{{NodeID: "node-a", Address: "10.0.0.1:50051"}},
			})
			return // abrupt drop: forces the resolver to reconnect
		}

		_ = conn.WriteJSON(brokerNodeEvent{
			Type:  "snapshot",
			Nodes: []brokerNode{{NodeID: "node-b", Address: "10.0.0.2:50051"}},
		})
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resolver := NewBrokerResolver(srv.URL, nil)
	ch, err := resolver.Watch(ctx)
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}

	first := awaitEvent(t, ch, 5*time.Second)
	if first.Identity.Key() != "local-node-a" {
		t.Fatalf("first event identity = %q, want local-node-a", first.Identity.Key())
	}

	// The reconnect happens after BrokerResolver's 1s minimum backoff.
	second := awaitEvent(t, ch, 5*time.Second)
	if second.Identity.Key() != "local-node-b" {
		t.Fatalf("second event identity = %q, want local-node-b", second.Identity.Key())
	}

	if attempts.Load() < 2 {
		t.Fatalf("connection attempts = %d, want >= 2 (resolver did not reconnect)", attempts.Load())
	}
}

// TestBrokerResolverUnavailableDoesNotCrashAndStopsOnCancel exercises
// invariant 6.3: the SDK keeps retrying quietly (no panic, no event, no
// closed channel) while the Broker is unreachable, and Watch's
// channel still closes promptly once ctx is cancelled mid-backoff.
func TestBrokerResolverUnavailableDoesNotCrashAndStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Port 1 is privileged and refuses connections immediately on loopback.
	resolver := NewBrokerResolver("http://127.0.0.1:1", nil)
	ch, err := resolver.Watch(ctx)
	if err != nil {
		t.Fatalf("Watch returned error for an unreachable Broker: %v", err)
	}

	select {
	case ev, ok := <-ch:
		if ok {
			t.Fatalf("unexpected event from unreachable Broker: %+v", ev)
		}
		t.Fatal("channel closed unexpectedly before context cancellation")
	case <-time.After(200 * time.Millisecond):
		// Expected: resolver is quietly retrying with backoff.
	}

	cancel()
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected channel close after cancel, got event")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("channel did not close after context cancellation")
	}
}
