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
			Nodes: []brokerNode{{NodeID: "node-shared", Address: "10.0.0.1:50051", Tasks: []string{"ocr"}}},
		})
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	assertSnapshot := func(t *testing.T, r NodeResolver) {
		t.Helper()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		ch, err := r.Watch(ctx)
		if err != nil {
			t.Fatalf("Watch: %v", err)
		}
		ev := awaitEvent(t, ch, 3*time.Second)
		if ev.Identity.Key() != "local-node-shared" {
			t.Fatalf("identity = %q, want local-node-shared", ev.Identity.Key())
		}
		if len(ev.Addresses) != 1 || ev.Addresses[0] != "10.0.0.1:50051" {
			t.Fatalf("addresses = %v, want [10.0.0.1:50051]", ev.Addresses)
		}
	}

	assertSnapshot(t, NewBrokerResolver(srv.URL, nil))
}
