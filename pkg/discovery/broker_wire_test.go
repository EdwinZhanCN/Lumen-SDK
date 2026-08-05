package discovery

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func awaitEvent(t *testing.T, ch <-chan NodeEvent, timeout time.Duration) NodeEvent {
	t.Helper()
	select {
	case event, ok := <-ch:
		if !ok {
			t.Fatal("event channel closed while waiting for event")
		}
		return event
	case <-time.After(timeout):
		t.Fatal("timed out waiting for event")
		return NodeEvent{}
	}
}

func TestParseNodeSnapshot(t *testing.T) {
	raw := []byte(`{
		"type": "snapshot",
		"nodes": [
			{"node_id": "node-a", "address": "10.0.0.1:50051", "txt": {"v": "1.0", "runtime": "burn", "tasks": "ocr", "proto": "9"}},
			{"node_id": "lab-node-b", "address": "[::1]:50052"}
		]
	}`)

	nodes, err := parseNodeSnapshot(raw, "lab")
	if err != nil {
		t.Fatalf("parseNodeSnapshot: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("got %d nodes, want 2", len(nodes))
	}
	if nodes[0].Identity.Key() != "lab-node-a" || nodes[0].Endpoint() != "10.0.0.1:50051" {
		t.Fatalf("unexpected first node: %+v", nodes[0])
	}
	if nodes[0].Txt["v"] != "1.0" || nodes[0].Txt["runtime"] != "burn" {
		t.Fatalf("descriptive metadata missing: %+v", nodes[0].Txt)
	}
	if _, ok := nodes[0].Txt["tasks"]; ok {
		t.Fatalf("task authority leaked through Broker TXT: %+v", nodes[0].Txt)
	}
	if _, ok := nodes[0].Txt["proto"]; ok {
		t.Fatalf("protocol authority leaked through Broker TXT: %+v", nodes[0].Txt)
	}
	if nodes[1].Identity.Key() != "lab-node-b" || nodes[1].Endpoint() != "[::1]:50052" {
		t.Fatalf("unexpected second node: %+v", nodes[1])
	}
}

func TestParseNodeSnapshotRejectsInvalidState(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{"malformed json", `{not valid json`},
		{"incremental event", `{"type":"added","nodes":[]}`},
		{"missing node id", `{"type":"snapshot","nodes":[{"address":"10.0.0.1:50051"}]}`},
		{"missing address", `{"type":"snapshot","nodes":[{"node_id":"node-a"}]}`},
		{"invalid address", `{"type":"snapshot","nodes":[{"node_id":"node-a","address":"not-an-endpoint"}]}`},
		{"duplicate identity", `{"type":"snapshot","nodes":[{"node_id":"node-a","address":"10.0.0.1:1"},{"node_id":"node-a","address":"10.0.0.2:2"}]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parseNodeSnapshot([]byte(tt.raw), DefaultDeploymentID); err == nil {
				t.Fatalf("parseNodeSnapshot(%q) succeeded", tt.raw)
			}
		})
	}
}

func TestBrokerWebSocketURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"http://localhost:5866", "ws://localhost:5866/v1/nodes/watch"},
		{"https://broker.example.com/base/", "wss://broker.example.com/base/v1/nodes/watch"},
		{"localhost:5866", "ws://localhost:5866/v1/nodes/watch"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := brokerWebSocketURL(tt.input)
			if err != nil {
				t.Fatalf("brokerWebSocketURL: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
	for _, input := range []string{"", "ftp://example.com", "http:///missing-host"} {
		if _, err := brokerWebSocketURL(input); err == nil {
			t.Fatalf("brokerWebSocketURL(%q) succeeded", input)
		}
	}
}

func TestBrokerResolverReconnectReplacesSnapshot(t *testing.T) {
	var attempts atomic.Int32
	upgrader := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := attempts.Add(1)
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		if attempt == 1 {
			_ = conn.WriteJSON(brokerNodeEvent{Type: "snapshot", Nodes: []brokerNode{{NodeID: "node-a", Address: "10.0.0.1:50051"}}})
			return
		}
		_ = conn.WriteJSON(brokerNodeEvent{Type: "snapshot", Nodes: []brokerNode{{NodeID: "node-b", Address: "10.0.0.2:50051"}}})
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, err := NewBrokerResolver(srv.URL, nil).Watch(ctx)
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}
	first := awaitEvent(t, ch, 5*time.Second)
	if first.Type != NodeDiscovered || first.Resolved.Key() != "local-node-a" {
		t.Fatalf("first event = %+v", first)
	}
	expired := awaitEvent(t, ch, 5*time.Second)
	if expired.Type != NodeExpired || expired.Resolved.Key() != "local-node-a" {
		t.Fatalf("replacement expiry = %+v", expired)
	}
	second := awaitEvent(t, ch, 5*time.Second)
	if second.Type != NodeDiscovered || second.Resolved.Key() != "local-node-b" {
		t.Fatalf("replacement discovery = %+v", second)
	}
	if attempts.Load() < 2 {
		t.Fatalf("connection attempts = %d, want >= 2", attempts.Load())
	}
}

func TestBrokerResolverUnavailableStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ch, err := NewBrokerResolver("http://127.0.0.1:1", nil).Watch(ctx)
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}
	select {
	case event, ok := <-ch:
		if ok {
			t.Fatalf("unexpected event: %+v", event)
		}
		t.Fatal("channel closed before cancellation")
	case <-time.After(200 * time.Millisecond):
	}
	cancel()
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected closed channel")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("channel did not close after cancellation")
	}
}

func TestBrokerResolverRejectsInvalidURL(t *testing.T) {
	_, err := NewBrokerResolver("ftp://example.com", nil).Watch(context.Background())
	if err == nil || !strings.Contains(err.Error(), "scheme") {
		t.Fatalf("invalid URL error = %v", err)
	}
}
