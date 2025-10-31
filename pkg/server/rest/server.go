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

// Server represents the REST API server
type Server struct {
	app    *fiber.App
	config *config.RESTConfig
}

// NewServer creates a new REST server instance
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

// Start starts the REST server
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

// HealthCheck returns server health status
func HealthCheck(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status":    "healthy",
		"timestamp": time.Now(),
		"service":   "Lumen REST API",
	})
}
