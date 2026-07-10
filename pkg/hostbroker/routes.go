package hostbroker

import (
	"github.com/gofiber/fiber/v2"
)

// setupRoutes registers the Host Broker's discovery-only route set:
// health, version, nodes, and nodes/watch. It must never register inference
// routes (/v1/infer, streaming, LLM/MCP endpoints) — that is the one hard
// invariant of this package.
func setupRoutes(app *fiber.App, watch *nodeWatchHub, version VersionInfo, catalog NodeCatalog) {
	v1 := app.Group("/v1")
	v1.Get("/health", healthHandler)
	v1.Get("/version", versionHandler(version))
	v1.Get("/nodes", nodesHandler(catalog))
	v1.Get("/nodes/watch", watch.upgrade)
}

func healthHandler(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(healthResponse{Status: "healthy"})
}

func versionHandler(version VersionInfo) fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusOK).JSON(version)
	}
}

func nodesHandler(catalog NodeCatalog) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if catalog == nil {
			return c.Status(fiber.StatusOK).JSON(nodesResponse{})
		}
		return c.Status(fiber.StatusOK).JSON(nodesResponse{Nodes: catalog.GetNodes()})
	}
}
