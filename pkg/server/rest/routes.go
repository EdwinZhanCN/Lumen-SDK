package rest

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
)

// SetupRoutes registers all REST API routes
func SetupRoutes(app *fiber.App, handlers Handlers) {
	if handlers == nil {
		panic(fmt.Errorf("handlers cannot be nil"))
	}

	// API v1 routes
	v1 := app.Group("/v1")

	// Health check
	v1.Get("/health", handlers.HealthCheck)

	// Inference endpoints
	v1.Post("/infer", handlers.Infer)

	// Service discovery endpoints
	v1.Get("/nodes", handlers.GetNodes)
	v1.Get("/nodes/:id/capabilities", handlers.GetNodeCapabilities)

	// Configuration endpoints
	v1.Get("/config", handlers.GetConfig)
	v1.Put("/config", handlers.UpdateConfig)
	v1.Get("/metrics", handlers.GetMetrics)
}
