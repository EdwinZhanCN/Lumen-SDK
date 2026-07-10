package service

import (
	"context"
	"fmt"
	"net"
	"runtime"
	"testing"
	"time"

	"github.com/edwinzhancn/lumen-sdk/cmd/lumen-hostd/internal"
	"github.com/edwinzhancn/lumen-sdk/pkg/config"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// internal.InitializeClient/CloseClient guard a package-level global, so
// HostdService tests cannot run in parallel and must reset that global
// around every test, including failed-Start cases. Ported from
// cmd/lumengatewayd/service/gatewayd_test.go (PR1) to keep the same
// characterization coverage after the rename to lumen-hostd.

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

func newTestService(t *testing.T) (*HostdService, *config.Config) {
	t.Helper()
	internal.ResetClient()
	t.Cleanup(func() { _ = internal.CloseClient() })

	cfg := newTestConfig(t)
	svc, err := NewHostdService(cfg, BuildInfo{Version: "test"}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewHostdService: %v", err)
	}
	return svc, cfg
}

func TestHostdServiceStartStopLifecycle(t *testing.T) {
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

func TestHostdServiceStartFailsWithoutDiscoveryBackend(t *testing.T) {
	internal.ResetClient()
	t.Cleanup(func() { _ = internal.CloseClient() })

	cfg := &config.Config{
		Discovery: config.DiscoveryConfig{Enabled: true}, // no mDNS, BrokerURL, or StaticNodes
		Server:    config.ServerConfig{REST: config.RESTConfig{Enabled: false}},
	}
	svc, err := NewHostdService(cfg, BuildInfo{Version: "test"}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewHostdService: %v", err)
	}

	if err := svc.Start(context.Background()); err == nil {
		t.Fatal("expected Start to fail with no discovery backend configured")
	}
	if svc.IsRunning() {
		t.Fatal("service must not report running after a failed Start")
	}
}

// TestHostdServiceInternalClientNeverUsesConfiguredBrokerURL locks down the
// self-subscription guard from plan §10: even if the caller's config sets a
// BrokerURL, the daemon's internal discovery client must not use it.
func TestHostdServiceInternalClientNeverUsesConfiguredBrokerURL(t *testing.T) {
	internal.ResetClient()
	t.Cleanup(func() { _ = internal.CloseClient() })

	cfg := newTestConfig(t)
	// Point at an address nothing is listening on. If the internal client
	// tried to use this, Start would hang trying to dial its own broker
	// websocket instead of just using the configured static node.
	cfg.Discovery.BrokerURL = "http://127.0.0.1:1"

	svc, err := NewHostdService(cfg, BuildInfo{Version: "test"}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewHostdService: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- svc.Start(ctx) }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Start did not return in time; internal client may be using the configured BrokerURL")
	}
	_ = svc.Stop()
}

func TestHostdServiceStopRejectsNewNodeWatchConnections(t *testing.T) {
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

func TestHostdServiceStopClosesExistingNodeWatchConnections(t *testing.T) {
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

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, _, err := conn.ReadMessage()
	if err == nil {
		t.Fatal("expected the watch connection to be closed after Stop")
	}
	if ne, ok := err.(net.Error); ok && ne.Timeout() {
		t.Fatalf("connection was not actively closed by Stop, it just timed out: %v", err)
	}

	after := stableGoroutineCount(t)
	if after > before {
		t.Fatalf("goroutine count rose from %d to %d; the watch connection's serve goroutine may not have exited", before, after)
	}
}

func TestHostdServiceStopDoesNotLeakGoroutines(t *testing.T) {
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
