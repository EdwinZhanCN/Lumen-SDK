package rest

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/edwinzhancn/lumen-sdk/pkg/config"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

// Server represents the REST API server for the Lumen SDK.
//
// The REST server provides HTTP endpoints for:
//   - ML inference operations (embedding, classification, face detection)
//   - Node management and discovery
//   - Health checks and metrics
//   - Configuration inspection
//
// Built on Fiber (FastHTTP), it offers high performance with low memory overhead.
//
// Role in project: Provides HTTP/REST interface for the Lumen SDK, enabling
// integration with web applications, microservices, and any HTTP-capable client.
// This is the primary interface for the lumenhubd daemon.
//
// Example:
//
//	cfg := &config.RESTConfig{
//	    Enabled: true,
//	    Host:    "0.0.0.0",
//	    Port:    5866,
//	    CORS:    true,
//	}
//	server := rest.NewServer(cfg)
type Server struct {
	app    *fiber.App
	config *config.RESTConfig
}

// NewServer creates a new REST API server instance with the specified configuration.
//
// This function initializes a Fiber application with:
//   - Error handling middleware
//   - Panic recovery middleware
//   - Optional CORS middleware
//   - Custom error handler for consistent error responses
//
// After creating the server, register routes using App() and then call Start().
//
// Parameters:
//   - cfg: REST server configuration (host, port, CORS, timeout)
//
// Returns:
//   - *Server: Initialized REST server ready for route registration
//
// Role in project: Factory function for creating the HTTP server. This is called
// by the daemon to set up the REST API interface.
//
// Example:
//
//	cfg := &config.RESTConfig{
//	    Enabled: true,
//	    Host:    "0.0.0.0",
//	    Port:    5866,
//	    CORS:    true,
//	    Timeout: 30 * time.Second,
//	}
//	server := rest.NewServer(cfg)
//
//	// Register routes
//	app := server.App()
//	app.Get("/v1/health", rest.HealthCheck)
//	app.Post("/v1/infer", inferHandler)
//
//	// Start server
//	if err := server.Start(); err != nil {
//	    log.Fatal(err)
//	}
func NewServer(cfg *config.RESTConfig) *Server {

	// Create Fiber app
	app := fiber.New(fiber.Config{
		AppName:               "Lumen SDK REST API",
		DisableStartupMessage: true,
		ErrorHandler:          errorHandler,
	})

	app.Use(recover.New())

	if cfg.CORS {
		app.Use(cors.New(cors.Config{
			AllowOrigins: "*",
			AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
			AllowHeaders: "Origin, Content-Type, Accept, Authorization",
		}))
	}

	return &Server{
		app:    app,
		config: cfg,
	}
}

// App returns the Fiber app instance for route registration
func (s *Server) App() *fiber.App {
	return s.app
}

// Start starts the REST API server and blocks until shutdown.
//
// The server listens on the configured host:port and handles requests until
// an interrupt signal (SIGINT/SIGTERM) is received. It implements graceful
// shutdown to allow in-flight requests to complete.
//
// This method blocks until the server is stopped, so it's typically called
// in the main goroutine or as the final step in daemon initialization.
//
// Returns:
//   - error: Non-nil if server fails to start or shutdown encounters issues
//
// Role in project: Starts the HTTP server and manages its lifecycle. This is
// the entry point for making the REST API available to clients.
//
// Example:
//
//	server := rest.NewServer(cfg)
//	// Register routes...
//	log.Println("Starting REST API server...")
//	if err := server.Start(); err != nil {
//	    log.Fatalf("Server error: %v", err)
//	}
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)

	log.Printf("Starting Lumen REST server on %s", addr)

	// Start server in goroutine to allow graceful shutdown
	go func() {
		if err := s.app.Listen(addr); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	return s.waitForShutdown()
}

// Stop gracefully stops the server
func (s *Server) Stop() error {
	log.Println("Shutting down REST server...")
	return s.app.Shutdown()
}

// waitForShutdown waits for interrupt signal and gracefully shuts down the server
func (s *Server) waitForShutdown() error {
	// Create channel to listen for interrupt signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Wait for interrupt signal
	<-quit
	log.Println("Received shutdown signal")

	// Gracefully shutdown server
	return s.Stop()
}

// errorHandler handles global errors
func errorHandler(c *fiber.Ctx, err error) error {
	// Default to 500 status code
	code := fiber.StatusInternalServerError

	// Retrieve the custom status code if it's a fiber.Error
	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
	}

	// Set Content-Type: application/json
	c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)

	// Return error response
	return c.Status(code).JSON(fiber.Map{
		"error":   true,
		"message": err.Error(),
		"code":    code,
	})
}

// HealthCheck is a handler that returns the server's health status.
//
// This endpoint is used for:
//   - Load balancer health checks
//   - Kubernetes liveness/readiness probes
//   - Monitoring systems
//   - Quick service availability verification
//
// Returns a JSON response with status, timestamp, and service name.
//
// Role in project: Provides health checking endpoint for operational monitoring
// and orchestration systems. Essential for production deployments.
//
// Example usage:
//
//	// Register the health check endpoint
//	app.Get("/v1/health", rest.HealthCheck)
//
//	// Client usage
//	curl http://localhost:5866/v1/health
//	// Response: {"status":"healthy","timestamp":"2024-01-01T12:00:00Z","service":"Lumen REST API"}
func HealthCheck(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status":    "healthy",
		"timestamp": time.Now(),
		"service":   "Lumen REST API",
	})
}
