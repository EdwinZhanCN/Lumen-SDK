package discovery

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// The compiler enforces this: PushResolver is a type alias (not a defined
// type) for BrokerResolver, so this assignment only compiles if they are
// identical types with no conversion.
var _ *BrokerResolver = (*PushResolver)(nil)

func TestNewPushResolverAndNewBrokerResolverReturnTheSameType(t *testing.T) {
	viaPush := NewPushResolver("http://localhost:5866", nil)
	viaBroker := NewBrokerResolver("http://localhost:5866", nil)

	// Both constructors must return *BrokerResolver: there is exactly one
	// implementation behind both names, not two independent resolvers.
	var _ *BrokerResolver = viaPush
	var _ *BrokerResolver = viaBroker

	if viaPush.brokerURL != viaBroker.brokerURL {
		t.Fatalf("brokerURL mismatch: push=%q broker=%q", viaPush.brokerURL, viaBroker.brokerURL)
	}
	if viaPush.deploymentID != viaBroker.deploymentID {
		t.Fatalf("deploymentID mismatch: push=%q broker=%q", viaPush.deploymentID, viaBroker.deploymentID)
	}
}

// TestPushAndBrokerConstructorsHitTheSameWireImplementation drives the exact
// same live /v1/nodes/watch server through both constructor names and
// confirms they observe identical events, proving PR2's claim: renaming to
// Broker terminology changed no networking or wire behavior.
func TestPushAndBrokerConstructorsHitTheSameWireImplementation(t *testing.T) {
	upgrader := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.WriteJSON(pushNodeEvent{
			Type:  "snapshot",
			Nodes: []pushNode{{NodeID: "node-shared", Address: "10.0.0.1:50051", Tasks: []string{"ocr"}}},
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

	t.Run("via NewPushResolver", func(t *testing.T) {
		assertSnapshot(t, NewPushResolver(srv.URL, nil))
	})
	t.Run("via NewBrokerResolver", func(t *testing.T) {
		assertSnapshot(t, NewBrokerResolver(srv.URL, nil))
	})
}
