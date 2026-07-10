package rest

import (
	"time"

	"github.com/edwinzhancn/lumen-sdk/pkg/client"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

// NewHandler is a thin compatibility adaptor used by gatewayd service startup.
//
// Historically gatewayd called rest.NewHandler(client, codecRegistry, logger).
// We keep the same signature but ignore codecRegistry for the unified REST
// server. This returns a Handlers implementation that the router will use.
func NewHandler(c *client.LumenClient, _ interface{}, logger *zap.Logger) Handlers {
	// Forward to the package-level constructor, keeping the caller's logger
	// for the node-watch hub.
	return newHandlers(c, logger)
}

// Router is a lightweight HTTP router wrapper used by the daemon startup code.
// It contains a Fiber app and the REST Handlers. It exposes a minimal surface:
// - SetupRoutes() to register routes
// - Start(addr string) to start listening
type Router struct {
	app     *fiber.App
	handler Handlers
	logger  *zap.Logger
}

// NewRouter constructs a Router that will register routes using the provided Handlers.
// The logger is optional; if nil, a no-op logger may be used.
func NewRouter(h Handlers, logger *zap.Logger) *Router {
	app := fiber.New(fiber.Config{
		AppName:               "Lumen Gatewayd REST Router",
		DisableStartupMessage: true,
	})

	r := &Router{
		app:     app,
		handler: h,
		logger:  logger,
	}
	return r
}

// SetupRoutes registers the package routes (delegates to routes.go)
func (r *Router) SetupRoutes() {
	SetupRoutes(r.app, r.handler)
}

// Start runs the HTTP server listening on the given address (host:port).
// It returns any error returned by Fiber's Listen (blocking call).
func (r *Router) Start(addr string) error {
	if r.logger != nil {
		r.logger.Info("Starting REST router", zap.String("address", addr))
	}
	return r.app.Listen(addr)
}

// App exposes the underlying Fiber app so callers can tweak middleware if needed.
func (r *Router) App() *fiber.App {
	return r.app
}

// Shutdown attempts to gracefully shutdown the server.
func (r *Router) Shutdown() error {
	if r.logger != nil {
		r.logger.Info("Shutting down REST router")
	}
	return r.app.Shutdown()
}

// ShutdownWithTimeout attempts to gracefully shutdown the server within timeout.
func (r *Router) ShutdownWithTimeout(timeout time.Duration) error {
	if r.logger != nil {
		r.logger.Info("Shutting down REST router", zap.Duration("timeout", timeout))
	}
	return r.app.ShutdownWithTimeout(timeout)
}

// watchCloser is implemented by Handlers that own hijacked WebSocket
// connections needing an explicit close on shutdown.
type watchCloser interface {
	Close()
}

// Close closes any node-watch clients still connected to the router's
// handler. Call this in addition to Shutdown/ShutdownWithTimeout, which only
// stop the listener and wait for in-flight regular HTTP requests: they do not
// close connections already hijacked for a WebSocket upgrade.
func (r *Router) Close() {
	if wc, ok := r.handler.(watchCloser); ok {
		wc.Close()
	}
}
