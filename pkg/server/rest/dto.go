package rest

type RESTInferRequest struct {
	Service       string            `json:"service" binding:"required"` // required service field for routing
	Task          string            `json:"task"`
	Payload       []byte            `json:"payload"`
	CorrelationID string            `json:"correlation_id"`
	Metadata      map[string]string `json:"metadata"`
}

// HealthResponse 健康检查响应
type HealthResponse struct {
	Status    string `json:"status"`
	Service   string `json:"service"`
	Timestamp int64  `json:"timestamp"` // 使用 Unix 时间戳也可以用 time.Time 并序列化为 RFC3339
}

// Node & capabilities 示例结构
type GetNodeCapabilitiesRequest struct {
	NodeID string `json:"node_id"`
}

// Config / UpdateConfig 示例结构
type Config struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type UpdateConfigRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// MetricsResponse 示例
type MetricsResponse struct {
	UptimeSeconds int64             `json:"uptime_seconds"`
	Stats         map[string]uint64 `json:"stats,omitempty"`
}
