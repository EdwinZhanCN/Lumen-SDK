package discovery

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestBrokerResolverReadsNodeWatch(t *testing.T) {
	upgrader := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.WriteJSON(brokerNodeEvent{
			Type:  "snapshot",
			Nodes: []brokerNode{{NodeID: "node-shared", Address: "10.0.0.1:50051"}},
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
	ch, err := NewBrokerResolver(srv.URL, nil).Watch(ctx)
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}
	event := awaitEvent(t, ch, 3*time.Second)
	if event.Type != NodeDiscovered || event.Resolved.Identity.Key() != "local-node-shared" {
		t.Fatalf("event = %+v", event)
	}
	if endpoint := event.Resolved.Endpoint(); endpoint != "10.0.0.1:50051" {
		t.Fatalf("endpoint = %q", endpoint)
	}
}

func TestEmitSnapshotReplacementUpdatesAddressAndExpiresMissingNodes(t *testing.T) {
	ctx := context.Background()
	ch := make(chan NodeEvent, 8)
	known := map[string]ResolvedNode{
		"local-node-a": {
			Identity:  NewNodeIdentity("local", "node-a"),
			Addresses: []string{"10.0.0.1"},
			Port:      50051,
		},
		"local-node-b": {
			Identity:  NewNodeIdentity("local", "node-b"),
			Addresses: []string{"10.0.0.2"},
			Port:      50051,
		},
	}
	snapshot := []ResolvedNode{{
		Identity:  NewNodeIdentity("local", "node-a"),
		Addresses: []string{"10.0.0.9"},
		Port:      50051,
	}}
	if err := emitSnapshotReplacement(ctx, ch, known, snapshot); err != nil {
		t.Fatalf("emitSnapshotReplacement: %v", err)
	}

	first := <-ch
	second := <-ch
	if first.Type != NodeExpired || first.Resolved.Key() != "local-node-b" {
		t.Fatalf("first event = %+v, want node-b expiry", first)
	}
	if second.Type != NodeDiscovered || second.Resolved.Endpoint() != "10.0.0.9:50051" {
		t.Fatalf("second event = %+v, want node-a address replacement", second)
	}
	if len(known) != 1 || known["local-node-a"].Endpoint() != "10.0.0.9:50051" {
		t.Fatalf("known state = %+v", known)
	}
}
