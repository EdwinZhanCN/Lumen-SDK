package rest

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	"github.com/edwinzhancn/lumen-sdk/pkg/client"
	pb "github.com/edwinzhancn/lumen-sdk/proto"
	"github.com/gofiber/fiber/v2"
)

// Handlers defines the interface for all REST API handlers
type Handlers interface {
	HealthCheck(c *fiber.Ctx) error
	Infer(c *fiber.Ctx) error
	GetNodes(c *fiber.Ctx) error
	GetNodeCapabilities(c *fiber.Ctx) error
	GetConfig(c *fiber.Ctx) error
	GetMetrics(c *fiber.Ctx) error
}

// handler implements the Handlers interface
type handler struct {
	client *client.LumenClient
	router *ServiceRouter
}

// NewHandlers creates a new Handlers instance
func NewHandlers(client *client.LumenClient) Handlers {
	return &handler{
		client: client,
		router: NewServiceRouter(client),
	}
}

func (h *handler) HealthCheck(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status":    "healthy",
		"service":   "Lumen REST API",
		"timestamp": time.Now(),
	})
}

func (h *handler) Infer(c *fiber.Ctx) error {
	var req RESTInferRequest

	// Support three incoming payload styles:
	// 1) multipart/form-data with file field `payload` (recommended for files)
	// 2) application/octet-stream raw body with service/task in query or headers
	// 3) application/json with base64 `payload` string (legacy)
	contentType := c.Get("Content-Type")

	if strings.HasPrefix(contentType, "multipart/form-data") {
		// Parse multipart form: payload as uploaded file
		fileHeader, err := c.FormFile("payload")
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   true,
				"message": "missing file payload: " + err.Error(),
			})
		}
		f, err := fileHeader.Open()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   true,
				"message": err.Error(),
			})
		}
		defer f.Close()

		buf := new(bytes.Buffer)
		if _, err := buf.ReadFrom(f); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   true,
				"message": err.Error(),
			})
		}
		req.Payload = buf.Bytes()
		req.Service = c.FormValue("service")
		req.Task = c.FormValue("task")
		req.CorrelationID = c.FormValue("correlation_id")
		// optional metadata passed as JSON string in form field `metadata`
		if metaStr := c.FormValue("metadata"); metaStr != "" {
			var meta map[string]string
			if err := json.Unmarshal([]byte(metaStr), &meta); err == nil {
				req.Metadata = meta
			}
		}
	} else if strings.HasPrefix(contentType, "application/octet-stream") {
		// Raw binary body. Service/task must be provided via query params or headers.
		req.Payload = c.Body()
		req.Service = c.Query("service")
		req.Task = c.Query("task")
		// correlation id may be provided as header X-Correlation-Id
		req.CorrelationID = c.Get("X-Correlation-Id")
		// optional metadata as JSON in query param `metadata`
		if metaStr := c.Query("metadata"); metaStr != "" {
			var meta map[string]string
			if err := json.Unmarshal([]byte(metaStr), &meta); err == nil {
				req.Metadata = meta
			}
		}
	} else {
		// Default: expect JSON body. In JSON, `payload` should be base64 encoded string that will be decoded by BodyParser into []byte.
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   true,
				"message": "Invalid request: " + err.Error(),
			})
		}
	}

	// normalize service name
	// serviceKey removed: router uses req.Service directly
	// Route request based on service field. The router may return either a
	// synchronous result (e.g. *pb.InferResponse) or a streaming channel
	// (<-chan *pb.InferResponse) for services whose names map to stream handlers.
	result, err := h.router.RouteRequest(c.Context(), req)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": err.Error(),
		})
	}

	// If the router returned a streaming channel, forward it as SSE (text/event-stream)
	if ch, ok := result.(<-chan *pb.InferResponse); ok {
		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")

		// Stream responses as JSON blocks; encode binary `Result` as base64.
		c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
			for resp := range ch {
				out := map[string]interface{}{
					"correlation_id": resp.CorrelationId,
					"is_final":       resp.IsFinal,
					"seq":            resp.Seq,
					"total":          resp.Total,
					"meta":           resp.Meta,
					"result_b64":     base64.StdEncoding.EncodeToString(resp.Result),
				}
				b, _ := json.Marshal(out)
				w.Write(b)
				w.WriteString("\n\n")
				w.Flush()

				if resp.IsFinal {
					break
				}
			}
		})

		return nil
	}

	// Otherwise return the synchronous result as JSON
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    result,
	})
}

func (h *handler) GetNodes(c *fiber.Ctx) error {
	nodes := h.client.GetNodes()

	return c.Status(fiber.StatusOK).JSON(nodes)
}

func (h *handler) GetNodeCapabilities(c *fiber.Ctx) error {
	var req GetNodeCapabilitiesRequest
	if err := c.BodyParser(&req); err != nil {
		return err
	}
	ctx := c.Context()
	caps, err := h.client.GetCapabilities(ctx, req.NodeID)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusOK).JSON(caps)
}

func (h *handler) GetConfig(c *fiber.Ctx) error {
	// Return the client's current effective configuration (thread-safe copy).
	// This avoids loading/parsing files again and ensures the view is consistent
	// with the running client.
	cfg := h.client.GetConfig()
	if cfg == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "configuration not available",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    cfg,
	})
}

func (h *handler) GetMetrics(c *fiber.Ctx) error {
	metrics := h.client.GetMetrics()
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    metrics,
	})
}
