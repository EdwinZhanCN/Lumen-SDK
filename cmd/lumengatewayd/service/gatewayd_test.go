package service

import (
	"context"
	"fmt"
	"net"
	"runtime"
	"testing"
	"time"

	"github.com/edwinzhancn/lumen-sdk/cmd/lumengatewayd/internal"
	"github.com/edwinzhancn/lumen-sdk/pkg/config"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// internal.InitializeClient/CloseClient guard a package-level global, so
// GatewaydService tests cannot run in parallel and must reset that global
// around every test, including failed-Start cases.

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find free port: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func newTestConfig(t *testing.T) *config.Config {
	t.Helper()
	return &config.Config{
		Discovery: config.DiscoveryConfig{
			Enabled:               true,
			DeploymentID:          "local",
			ResolveTimeout:        200 * time.Millisecond,
			ConnectTimeout:        200 * time.Millisecond,
			RediscoveryBackoffMin: 200 * time.Millisecond,
			RediscoveryBackoffMax: 500 * time.Millisecond,
			// Port 1 refuses connections immediately on loopback, so the pool
			// keeps retrying without ever going ready, without needing a real
			// Lumen node in the test environment.
			StaticNodes: []string{"127.0.0.1:1"},
		},
		Server: config.ServerConfig{
			REST: config.RESTConfig{
				Enabled: true,
				Host:    "127.0.0.1",
				Port:    freePort(t),
			},
		},
	}
}

func newTestService(t *testing.T) (*GatewaydService, *config.Config) {
	t.Helper()
	internal.ResetClient()
	t.Cleanup(func() { _ = internal.CloseClient() })

	cfg := newTestConfig(t)
	svc, err := NewGatewaydService(cfg, zap.NewNop())
	if err != nil {
		t.Fatalf("NewGatewaydService: %v", err)
	}
	return svc, cfg
}

func TestGatewaydServiceStartStopLifecycle(t *testing.T) {
	svc, _ := newTestService(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if svc.IsRunning() {
		t.Fatal("service reports running before Start")
	}

	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !svc.IsRunning() {
		t.Fatal("service does not report running after Start")
	}
	if svc.GetUptime() <= 0 {
		t.Fatal("expected positive uptime after Start")
	}
	if status := svc.GetStatus(); status["running"] != true {
		t.Fatalf("status[running] = %v, want true", status["running"])
	}

	if err := svc.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if svc.IsRunning() {
		t.Fatal("service still reports running after Stop")
	}
	if svc.GetUptime() != 0 {
		t.Fatalf("uptime after Stop = %v, want 0", svc.GetUptime())
	}
}

// TestGatewaydServiceStartFailsWithoutDiscoveryBackend locks down the "no
// discovery backend configured" error path from client.NewLumenClient and
// confirms a failed Start never leaves IsRunning() reporting true.
func TestGatewaydServiceStartFailsWithoutDiscoveryBackend(t *testing.T) {
	internal.ResetClient()
	t.Cleanup(func() { _ = internal.CloseClient() })

	cfg := &config.Config{
		Discovery: config.DiscoveryConfig{Enabled: true}, // no mDNS, BrokerURL, or StaticNodes
		Server:    config.ServerConfig{REST: config.RESTConfig{Enabled: false}},
	}
	svc, err := NewGatewaydService(cfg, zap.NewNop())
	if err != nil {
		t.Fatalf("NewGatewaydService: %v", err)
	}

	if err := svc.Start(context.Background()); err == nil {
		t.Fatal("expected Start to fail with no discovery backend configured")
	}
	if svc.IsRunning() {
		t.Fatal("service must not report running after a failed Start")
	}
}

// TestGatewaydServiceStopRejectsNewNodeWatchConnections covers the half of
// Milestone 0's "closing the daemon closes watchers" criterion that actually
// holds today: ShutdownWithTimeout stops the listener, so no new /v1/nodes/watch
// client can connect once Stop has returned.
//
// This must wait for the REST listener to actually be up before calling Stop:
// startServers launches router.Start(addr) in a goroutine and Start() returns
// without waiting for it to bind, so calling Stop immediately after Start
// races the listener's own startup — Shutdown can complete before Listen has
// bound the port, which then binds *after* the daemon considers itself
// stopped. That startup race is real but is a separate concern from the
// shutdown behavior this test targets.
func TestGatewaydServiceStopRejectsNewNodeWatchConnections(t *testing.T) {
	svc, cfg := newTestService(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	wsURL := fmt.Sprintf("ws://%s:%d/v1/nodes/watch", cfg.Server.REST.Host, cfg.Server.REST.Port)
	waitForCondition(t, func() bool {
		c, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			return false
		}
		c.Close()
		return true
	})

	if err := svc.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	if _, _, err := websocket.DefaultDialer.Dial(wsURL, nil); err == nil {
		t.Fatal("expected dial to fail once the daemon has stopped")
	}
}

// TestGatewaydServiceStopClosesExistingNodeWatchConnections is the other half
// of Milestone 0's "closing the daemon closes watchers" criterion. It used to
// fail: fiber/fasthttp's ShutdownWithTimeout does not track hijacked
// connections (the /v1/nodes/watch WebSocket upgrade), so an already-connected
// client stayed open — and its nodeWatchHub.serve goroutine blocked in
// ReadMessage — until the process itself exited. GatewaydService.Stop now
// also calls router.Close, which closes those connections directly.
func TestGatewaydServiceStopClosesExistingNodeWatchConnections(t *testing.T) {
	svc, cfg := newTestService(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	wsURL := fmt.Sprintf("ws://%s:%d/v1/nodes/watch", cfg.Server.REST.Host, cfg.Server.REST.Port)
	var conn *websocket.Conn
	waitForCondition(t, func() bool {
		c, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			return false
		}
		conn = c
		return true
	})
	defer conn.Close()
	if _, _, err := conn.ReadMessage(); err != nil {
		t.Fatalf("read snapshot: %v", err)
	}

	before := stableGoroutineCount(t)

	if err := svc.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	// A close (not a client-side timeout) must arrive well within the old
	// gap's window; a generous deadline just guards against a hang.
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, _, err := conn.ReadMessage()
	if err == nil {
		t.Fatal("expected the watch connection to be closed after Stop")
	}
	if ne, ok := err.(net.Error); ok && ne.Timeout() {
		t.Fatalf("connection was not actively closed by Stop, it just timed out: %v", err)
	}

	// Stop's other teardown (gRPC pool, etc.) already reduces the goroutine
	// count on its own; if the watcher's serve goroutine also exited as
	// expected, the count must not have increased.
	after := stableGoroutineCount(t)
	if after > before {
		t.Fatalf("goroutine count rose from %d to %d; the watch connection's serve goroutine may not have exited", before, after)
	}
}

// TestGatewaydServiceStopDoesNotLeakGoroutines covers the Milestone 0
// shutdown acceptance criterion for the case with no connected watchers; see
// TestGatewaydServiceStopClosesExistingNodeWatchConnections for the
// connected-watcher case.
func TestGatewaydServiceStopDoesNotLeakGoroutines(t *testing.T) {
	baseline := stableGoroutineCount(t)

	svc, _ := newTestService(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := svc.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	after := stableGoroutineCount(t)
	if after > baseline+5 {
		t.Fatalf("goroutine count after Stop = %d, want <= baseline(%d)+5", after, baseline)
	}
}

// stableGoroutineCount polls runtime.NumGoroutine until two consecutive
// samples agree, since connection teardown (gRPC ClientConn, websocket read
// loops) finishes asynchronously on other goroutines.
func stableGoroutineCount(t *testing.T) int {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	prev := runtime.NumGoroutine()
	for time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
		cur := runtime.NumGoroutine()
		if cur == prev {
			return cur
		}
		prev = cur
	}
	return prev
}

func waitForCondition(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("condition not met within deadline")
}
