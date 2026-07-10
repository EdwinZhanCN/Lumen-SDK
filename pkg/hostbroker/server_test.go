package hostbroker

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/edwinzhancn/lumen-sdk/pkg/discovery"
	pb "github.com/edwinzhancn/lumen-sdk/proto"
	"github.com/gorilla/websocket"
)

// fakeCatalog is a minimal, directly-controllable NodeCatalog test double.
type fakeCatalog struct {
	mu       sync.RWMutex
	nodes    []*discovery.NodeInfo
	watchers []func([]*discovery.NodeInfo)
}

func (f *fakeCatalog) GetNodes() []*discovery.NodeInfo {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.nodes
}

func (f *fakeCatalog) WatchNodes(cb func([]*discovery.NodeInfo)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.watchers = append(f.watchers, cb)
}

func (f *fakeCatalog) set(nodes []*discovery.NodeInfo) {
	f.mu.Lock()
	f.nodes = nodes
	watchers := append([]func([]*discovery.NodeInfo){}, f.watchers...)
	f.mu.Unlock()
	for _, w := range watchers {
		w(nodes)
	}
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

// startTestServer starts a Server on an ephemeral port and returns it plus
// the base HTTP URL.
func startTestServer(t *testing.T, catalog NodeCatalog) (*Server, string) {
	t.Helper()

	srv := NewServer(catalog, VersionInfo{Version: "test"}, nil)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() { _ = srv.App().Listener(ln) }()
	t.Cleanup(func() { _ = srv.Shutdown() })

	return srv, fmt.Sprintf("http://%s", ln.Addr().String())
}

func TestServerHealthEndpoint(t *testing.T) {
	_, baseURL := startTestServer(t, nil)

	resp, err := http.Get(baseURL + "/v1/health")
	if err != nil {
		t.Fatalf("GET /v1/health: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var body healthResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Status != "healthy" {
		t.Fatalf("status = %q, want healthy", body.Status)
	}
}

func TestServerVersionEndpoint(t *testing.T) {
	srv := NewServer(nil, VersionInfo{Version: "1.2.3", Commit: "abc123", BuildTime: "2026-07-10T00:00:00Z"}, nil)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() { _ = srv.App().Listener(ln) }()
	t.Cleanup(func() { _ = srv.Shutdown() })

	resp, err := http.Get(fmt.Sprintf("http://%s/v1/version", ln.Addr().String()))
	if err != nil {
		t.Fatalf("GET /v1/version: %v", err)
	}
	defer resp.Body.Close()
	var body VersionInfo
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Version != "1.2.3" || body.Commit != "abc123" {
		t.Fatalf("version = %+v, want Version=1.2.3 Commit=abc123", body)
	}
}

func TestServerNodesEndpoint(t *testing.T) {
	catalog := &fakeCatalog{nodes: []*discovery.NodeInfo{activeNode("node-a", "10.0.0.1:50051", "ocr")}}
	_, baseURL := startTestServer(t, catalog)

	resp, err := http.Get(baseURL + "/v1/nodes")
	if err != nil {
		t.Fatalf("GET /v1/nodes: %v", err)
	}
	defer resp.Body.Close()
	var body nodesResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Nodes) != 1 || body.Nodes[0].ID != "node-a" {
		t.Fatalf("nodes = %+v, want [node-a]", body.Nodes)
	}
}

// TestServerDoesNotExposeInferenceRoutes is the one hard invariant of this
// package: a discovery-only Broker must never register /v1/infer or other
// inference-facing routes, even by accident in a future edit.
func TestServerDoesNotExposeInferenceRoutes(t *testing.T) {
	_, baseURL := startTestServer(t, nil)

	for _, path := range []string{"/v1/infer", "/v1/infer/stream", "/v1/tools", "/v1/mcp"} {
		resp, err := http.Post(baseURL+path, "application/json", nil)
		if err != nil {
			t.Fatalf("POST %s: %v", path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("POST %s status = %d, want 404 (route must not exist)", path, resp.StatusCode)
		}
	}
}

func TestServerNodesWatchServesBrokerResolver(t *testing.T) {
	catalog := &fakeCatalog{}
	srv, baseURL := startTestServer(t, catalog)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resolver := discovery.NewBrokerResolver(baseURL, nil)
	events, err := resolver.Watch(ctx)
	if err != nil {
		t.Fatalf("resolver watch: %v", err)
	}

	// Wait for the WS client to register (and so WatchNodes to be called on
	// the catalog) before mutating it, or the update could fire before
	// anyone is listening and be lost.
	waitFor(t, func() bool {
		srv.watch.mu.Lock()
		defer srv.watch.mu.Unlock()
		return len(srv.watch.clients) == 1
	})

	// Initial (empty) snapshot, then a node appears.
	catalog.set([]*discovery.NodeInfo{activeNode("node-b", "10.0.0.2:50051", "ocr")})

	ev := nextEvent(t, events)
	if ev.Type != discovery.NodeDiscovered {
		t.Fatalf("event type = %v, want NodeDiscovered", ev.Type)
	}
	if ev.Identity.Key() != "local-node-b" {
		t.Fatalf("identity = %q, want local-node-b", ev.Identity.Key())
	}

	catalog.set(nil)
	ev = nextEvent(t, events)
	if ev.Type != discovery.NodeExpired || !ev.ExplicitRemove {
		t.Fatalf("expected explicit NodeExpired, got type=%v explicit=%v", ev.Type, ev.ExplicitRemove)
	}
}

func TestServerCloseDisconnectsWatchClients(t *testing.T) {
	srv, baseURL := startTestServer(t, &fakeCatalog{})
	wsURL := "ws" + baseURL[len("http"):] + "/v1/nodes/watch"

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	if _, _, err := conn.ReadMessage(); err != nil {
		t.Fatalf("read snapshot: %v", err)
	}

	waitFor(t, func() bool {
		srv.watch.mu.Lock()
		defer srv.watch.mu.Unlock()
		return len(srv.watch.clients) == 1
	})

	srv.Close()

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, _, err := conn.ReadMessage(); err == nil {
		t.Fatal("expected the connection to be closed by Server.Close")
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
