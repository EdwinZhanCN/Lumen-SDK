package rest

import (
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
	GetTasks(c *fiber.Ctx) error
}

// handler implements the Handlers interface
type handler struct {
	client *client.LumenClient
}

// NewHandlers creates a new Handlers instance
func NewHandlers(client *client.LumenClient) Handlers {
	return &handler{
		client: client,
	}
}

func (h *handler) HealthCheck(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(APIResponse{
		Success:   true,
		Timestamp: time.Now().Format(time.RFC3339),
		Data: fiber.Map{
			"status":  "healthy",
			"service": "Lumen REST API",
		},
	})
}

func (h *handler) Infer(c *fiber.Ctx) error {
	var req RESTInferRequest
	var payload []byte

	// Support three incoming payload styles:
	// 1) multipart/form-data with file field `payload` (recommended for files)
	// 2) application/octet-stream raw body with task/payload_mime/meta in query or headers
	// 3) application/json using the Lumen Hub InferRequest envelope
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
		payload = buf.Bytes()
		req.Task = c.FormValue("task")
		req.PayloadMime = c.FormValue("payload_mime")
		req.CorrelationID = c.FormValue("correlation_id")
		if metaStr := c.FormValue("meta"); metaStr != "" {
			var meta map[string]string
			if err := json.Unmarshal([]byte(metaStr), &meta); err == nil {
				req.Meta = meta
			}
		}
	} else if strings.HasPrefix(contentType, "application/octet-stream") {
		// Raw binary body. Task/payload_mime/meta must be provided via query params or headers.
		payload = c.Body()
		req.Task = c.Query("task")
		req.PayloadMime = c.Query("payload_mime", contentType)
		req.CorrelationID = c.Get("X-Correlation-Id")
		if metaStr := c.Query("meta"); metaStr != "" {
			var meta map[string]string
			if err := json.Unmarshal([]byte(metaStr), &meta); err == nil {
				req.Meta = meta
			}
		}
	} else {
		// Default: expect JSON body. In JSON, text/plain payload is raw UTF-8 text;
		// binary payloads are base64 strings.
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   true,
				"message": "Invalid request: " + err.Error(),
			})
		}
		if req.PayloadMime == "text/plain" {
			payload = []byte(req.Payload)
		} else {
			decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(req.Payload))
			if err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"error":   true,
					"message": "Invalid base64 payload: " + err.Error(),
				})
			}
			payload = decoded
		}
	}

	inferReq := &pb.InferRequest{
		CorrelationId: req.CorrelationID,
		Task:          req.Task,
		Payload:       payload,
		PayloadMime:   req.PayloadMime,
		Meta:          req.Meta,
		Seq:           req.Seq,
		Total:         req.Total,
		Offset:        req.Offset,
	}
	result, err := h.client.Infer(c.Context(), inferReq)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": err.Error(),
		})
	}

	// Otherwise return the synchronous result as JSON
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    result,
	})
}

func (h *handler) GetNodes(c *fiber.Ctx) error {
	nodes := h.client.GetNodes()

	return c.Status(fiber.StatusOK).JSON(APIResponse{
		Success: true,
		Data: fiber.Map{
			"nodes": nodes,
		},
	})
}

func (h *handler) GetNodeCapabilities(c *fiber.Ctx) error {
	var req GetNodeCapabilitiesRequest
	if err := c.BodyParser(&req); err != nil {
		return err
	}

	nodes := h.client.GetNodes()
	var caps []*pb.Capability
	for _, n := range nodes {
		if n.ID == req.NodeID {
			caps = n.Capabilities
			break
		}
	}

	return c.Status(fiber.StatusOK).JSON(APIResponse{
		Success: true,
		Data:    caps,
	})
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

func (h *handler) GetTasks(c *fiber.Ctx) error {
	// Get all nodes from existing client
	nodes := h.client.GetNodes()

	// TaskSummary for JSON response
	type TaskSummary struct {
		Name        string   `json:"name"`
		InputMimes  []string `json:"input_mimes,omitempty"`
		OutputMimes []string `json:"output_mime,omitempty"`
		NodeID      string   `json:"node_id"`
		NodeName    string   `json:"node_name"`
		ServiceName string   `json:"service_name"`
		NodeAddress string   `json:"node_address"`
	}

	// Group tasks by service
	serviceTasks := make(map[string][]TaskSummary)
	var allTasks []TaskSummary
	activeNodes := 0

	for _, node := range nodes {
		if !node.IsActive() {
			continue
		}
		activeNodes++

		// Process Tasks field from NodeInfo
		for _, task := range node.Tasks {
			summary := TaskSummary{
				Name:        task.Name,
				InputMimes:  task.InputMimes,
				OutputMimes: task.OutputMimes,
				NodeID:      node.ID,
				NodeName:    node.ID,
				NodeAddress: node.Address,
				ServiceName: "", // Will be filled from Capabilities
			}

			// Find service name from Capabilities
			for _, capability := range node.Capabilities {
				for _, capabilityTask := range capability.Tasks {
					if capabilityTask.Name == task.Name {
						summary.ServiceName = capability.ServiceName
						break
					}
				}
				if summary.ServiceName != "" {
					break
				}
			}

			// If still empty, use a default
			if summary.ServiceName == "" {
				summary.ServiceName = "unknown"
			}

			serviceTasks[summary.ServiceName] = append(serviceTasks[summary.ServiceName], summary)
			allTasks = append(allTasks, summary)
		}
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"services":       serviceTasks,
			"all_tasks":      allTasks,
			"total_nodes":    len(nodes),
			"active_nodes":   activeNodes,
			"total_tasks":    len(allTasks),
			"services_count": len(serviceTasks),
		},
		"timestamp": time.Now(),
	})
}
