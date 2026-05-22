package rest

// RESTInferRequest mirrors the gRPC InferRequest envelope for REST callers.
// For text/plain, Payload is UTF-8 text. For binary MIME types, Payload is base64.
type RESTInferRequest struct {
	Task          string            `json:"task"`
	PayloadMime   string            `json:"payload_mime"`
	Payload       string            `json:"payload"`
	CorrelationID string            `json:"correlation_id"`
	Meta          map[string]string `json:"meta"`
	Seq           uint64            `json:"seq,omitempty"`
	Total         uint64            `json:"total,omitempty"`
	Offset        uint64            `json:"offset,omitempty"`
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
