// Package hostbroker is the Lumen Host Broker's discovery-only server
// surface: health, version, node listing, and the /v1/nodes/watch
// push-discovery endpoint consumed by discovery.BrokerResolver.
//
// It deliberately has no inference route surface (no /v1/infer, no
// streaming, no LLM/MCP endpoints) — the Broker is a control-plane-only
// process that reports node identities and endpoints; it never sees
// inference payloads. See docs/lumen-host-implementation-plan.md §10.
package hostbroker

import (
	"time"

	"github.com/edwinzhancn/lumen-sdk/pkg/discovery"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

// NodeCatalog is the minimal read-only view pkg/hostbroker needs from a node
// registry. *client.LumenClient satisfies this today without an adapter.
// Keeping the dependency at interface level — rather than importing
// pkg/client directly — leaves room for a lighter discovery-only catalog
// later without an API break here.
type NodeCatalog interface {
	GetNodes() []*discovery.NodeInfo
	WatchNodes(cb func([]*discovery.NodeInfo))
}

// VersionInfo is build-time version metadata surfaced at GET /v1/version.
// Callers populate this from ldflags-injected main package variables.
type VersionInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"build_time"`
}

// Server is the Host Broker's HTTP/WebSocket surface.
type Server struct {
	app    *fiber.App
	watch  *nodeWatchHub
	logger *zap.Logger
}

// NewServer constructs a Server. catalog may be nil only in tests exercising
// the health/version routes in isolation; production callers must pass a
// real NodeCatalog.
func NewServer(catalog NodeCatalog, version VersionInfo, logger *zap.Logger) *Server {
	if logger == nil {
		logger = zap.NewNop()
	}
	app := fiber.New(fiber.Config{
		AppName:               "Lumen Host Broker",
		DisableStartupMessage: true,
	})

	s := &Server{
		app:    app,
		watch:  newNodeWatchHub(catalog, logger),
		logger: logger,
	}
	setupRoutes(app, s.watch, version, catalog)
	return s
}

// App exposes the underlying Fiber app, e.g. for tests that need to attach a
// pre-bound listener.
func (s *Server) App() *fiber.App {
	return s.app
}

// Start runs the HTTP server, blocking until it stops or fails.
func (s *Server) Start(addr string) error {
	s.logger.Info("starting Host Broker server", zap.String("address", addr))
	return s.app.Listen(addr)
}

// Shutdown gracefully stops the server: no new connections, waits for
// in-flight regular HTTP requests. It does not close already-hijacked
// /v1/nodes/watch connections; call Close for that.
func (s *Server) Shutdown() error {
	return s.app.Shutdown()
}

// ShutdownWithTimeout is Shutdown bounded by timeout.
func (s *Server) ShutdownWithTimeout(timeout time.Duration) error {
	return s.app.ShutdownWithTimeout(timeout)
}

// Close closes any connected /v1/nodes/watch clients. fiber/fasthttp's
// graceful shutdown does not track hijacked WebSocket connections, so
// without this an already-connected watcher would stay open — and its
// per-connection goroutine blocked — until the process itself exits rather
// than when the Broker stops. Call this alongside Shutdown/ShutdownWithTimeout,
// not instead of it.
func (s *Server) Close() {
	s.watch.Close()
}
