package rest

// RESTInferRequest is the unified inference request DTO used by the REST API.
// Clients should populate `Service` (used for routing) and supply the binary
// `Payload` (in JSON use base64; multipart and octet-stream modes are also supported).
type RESTInferRequest struct {
	Service       string            `json:"service" binding:"required"` // required service field for routing
	Task          string            `json:"task"`
	Payload       []byte            `json:"payload"`
	CorrelationID string            `json:"correlation_id"`
	Metadata      map[string]string `json:"metadata"`
}

// APIError represents an error object returned by the REST API.
type APIError struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

// APIResponse is the unified response envelope returned by the REST API.
// Handlers should populate Success, optional Error and Data fields.
// Timestamp and RequestID are optional helpers for clients/CLI.
type APIResponse struct {
	Success   bool        `json:"success"`
	Error     *APIError   `json:"error,omitempty"`
	Data      interface{} `json:"data,omitempty"`
	Timestamp string      `json:"timestamp,omitempty"`
	RequestID string      `json:"request_id,omitempty"`
}

// Node & capabilities 示例结构
type GetNodeCapabilitiesRequest struct {
	NodeID string `json:"node_id"`
}
