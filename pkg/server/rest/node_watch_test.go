package rest

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/edwinzhancn/lumen-sdk/pkg/discovery"
	pb "github.com/edwinzhancn/lumen-sdk/proto"
	"github.com/gofiber/fiber/v2"
)

// startWatchServer runs a fiber app with only the node-watch route on an
// ephemeral port and returns the hub plus the base http URL.
func startWatchServer(t *testing.T) (*nodeWatchHub, string) {
	t.Helper()

	hub := newNodeWatchHub(nil, nil)
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Get("/v1/nodes/watch", hub.upgrade)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() { _ = app.Listener(ln) }()
	t.Cleanup(func() { _ = app.Shutdown() })

	return hub, fmt.Sprintf("http://%s", ln.Addr().String())
}

func activeNode(id, addr string, tasks ...string) *discovery.NodeInfo {
	ioTasks := make([]*pb.IOTask, 0, len(tasks))
	for _, task := range tasks {
		ioTasks = append(ioTasks, &pb.IOTask{Name: task})
	}
	return &discovery.NodeInfo{
		ID:      id,
		Address: addr,
		Status:  discovery.NodeStatusActive,
		Tasks:   ioTasks,
	}
}

// TestNodeWatchServesPushResolver is the B3 regression test: the shared REST
// route must be consumable by discovery.PushResolver end to end.
func TestNodeWatchServesPushResolver(t *testing.T) {
	hub, baseURL := startWatchServer(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resolver := discovery.NewPushResolver(baseURL, nil)
	events, err := resolver.Watch(ctx)
	if err != nil {
		t.Fatalf("push resolver watch: %v", err)
	}

	// Wait until the WS client is registered, then push an added diff.
	waitFor(t, func() bool {
		hub.mu.Lock()
		defer hub.mu.Unlock()
		return len(hub.clients) == 1
	})
	hub.broadcast([]*discovery.NodeInfo{activeNode("local-node-a", "10.0.0.9:50051", "ocr")})

	ev := nextEvent(t, events)
	if ev.Type != discovery.NodeDiscovered {
		t.Fatalf("event type = %v, want NodeDiscovered", ev.Type)
	}
	if ev.Identity.Key() != "local-node-a" {
		t.Fatalf("identity = %q, want local-node-a", ev.Identity.Key())
	}
	if len(ev.Addresses) == 0 || ev.Addresses[0] != "10.0.0.9:50051" {
		t.Fatalf("addresses = %v, want [10.0.0.9:50051]", ev.Addresses)
	}
	if len(ev.Tasks) != 1 || ev.Tasks[0] != "ocr" {
		t.Fatalf("tasks = %v, want [ocr]", ev.Tasks)
	}

	// Node disappears -> explicit removal event.
	hub.broadcast(nil)
	ev = nextEvent(t, events)
	if ev.Type != discovery.NodeExpired || !ev.ExplicitRemove {
		t.Fatalf("expected explicit NodeExpired, got type=%v explicit=%v", ev.Type, ev.ExplicitRemove)
	}
	if ev.Identity.Key() != "local-node-a" {
		t.Fatalf("removed identity = %q, want local-node-a", ev.Identity.Key())
	}
}

func TestNodeWatchBroadcastDiffsOnlyChanges(t *testing.T) {
	hub, _ := startWatchServer(t)

	// No clients: broadcast must still advance prevNodes without panicking.
	hub.broadcast([]*discovery.NodeInfo{activeNode("local-n1", "1.1.1.1:1", "ocr")})
	hub.mu.Lock()
	_, tracked := hub.prevNodes["local-n1"]
	hub.mu.Unlock()
	if !tracked {
		t.Fatal("broadcast should track active nodes even without clients")
	}

	// Inactive nodes are excluded from the active set.
	hub.broadcast([]*discovery.NodeInfo{{ID: "local-n1", Status: discovery.NodeStatusError}})
	hub.mu.Lock()
	_, tracked = hub.prevNodes["local-n1"]
	hub.mu.Unlock()
	if tracked {
		t.Fatal("inactive node should be dropped from the tracked set")
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not met within deadline")
}

func nextEvent(t *testing.T, events <-chan discovery.NodeEvent) discovery.NodeEvent {
	t.Helper()
	select {
	case ev, ok := <-events:
		if !ok {
			t.Fatal("event channel closed")
		}
		return ev
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for node event")
		return discovery.NodeEvent{}
	}
}
