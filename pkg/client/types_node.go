package client

import (
	"sync"
	"sync/atomic"
	"time"

	pb "github.com/edwinzhancn/lumen-sdk/proto"
)

// NodeInfo represents a discovered ML inference node with its capabilities and status.
//
// This structure contains all information about an ML node including:
//   - Identity and location (ID, Name, Address)
//   - Current status and last contact time
//   - ML capabilities (supported tasks, models, hardware)
//   - Performance metrics and load information
//   - Load balancing metadata (weight, connections)
//
// NodeInfo is used by:
//   - Service discovery to track available nodes
//   - Load balancers to select appropriate nodes
//   - Connection pools to manage node connections
//   - Monitoring systems to track cluster health
//
// Role in project: Central data structure representing ML nodes in the distributed
// cluster. Essential for discovery, routing, load balancing, and monitoring.
//
// Example:
//
//	nodes := client.GetNodes()
//	for _, node := range nodes {
//	    if node.IsActive() && node.SupportsTask("embedding") {
//	        fmt.Printf("Node %s: %s at %s\n",
//	            node.ID, node.Name, node.Address)
//	        fmt.Printf("Runtime: %s, Models: %d\n",
//	            node.Runtime, len(node.Models))
//	    }
//	}
type NodeInfo struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Address      string                 `json:"address"`
	Status       NodeStatus             `json:"status"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	Capabilities []*pb.Capability       `json:"capabilities,omitempty"`
	Version      string                 `json:"version"`
	Runtime      string                 `json:"runtime"`
	Models       []*ModelInfo           `json:"models,omitempty"`
	LastSeen     time.Time              `json:"last_seen"`
	Tasks        []*pb.IOTask           `json:"tasks,omitempty"`

	// Load balancing fields
	Weight         int64           `json:"weight"`
	Load           *NodeLoad       `json:"load,omitempty"`
	Stats          *NodeStats      `json:"stats,omitempty"`
	connections    int64           `json:"-"` // 当前连接数
	supportedTasks map[string]bool `json:"-"` // 支持的任务缓存
	mu             sync.RWMutex    `json:"-"` // 读写锁
}

type ModelInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version"`
	Runtime string `json:"runtime"`
}

// NodeStatus represents the current operational state of an ML node.
//
// Role in project: Tracks node lifecycle for health monitoring and routing decisions.
type NodeStatus string

const (
	NodeStatusUnknown  NodeStatus = "unknown"  // Initial state or communication lost
	NodeStatusStarting NodeStatus = "starting" // Node is initializing
	NodeStatusActive   NodeStatus = "active"   // Node is healthy and accepting requests
	NodeStatusError    NodeStatus = "error"    // Node encountered an error
)

// NodeLoad represents real-time resource utilization of an ML node.
//
// All values are ratios from 0.0 (idle) to 1.0 (fully utilized).
// Load information helps load balancers make intelligent routing decisions
// to avoid overloading nodes.
//
// Role in project: Provides resource utilization data for intelligent load
// balancing and capacity planning.
type NodeLoad struct {
	CPU    float64 `json:"cpu"`    // CPU 使用率 0-1
	Memory float64 `json:"memory"` // 内存使用率 0-1
	GPU    float64 `json:"gpu"`    // GPU 使用率 0-1
	Disk   float64 `json:"disk"`   // 磁盘使用率 0-1
}

// NodeStats tracks performance metrics for an ML node.
//
// These statistics help with:
//   - Monitoring node health and performance
//   - Load balancing decisions based on success rate
//   - Identifying problematic nodes
//   - Calculating error rates and latencies
//
// Role in project: Provides operational metrics for monitoring, alerting,
// and intelligent routing decisions.
type NodeStats struct {
	TotalRequests      int64     `json:"total_requests"`
	SuccessfulRequests int64     `json:"successful_requests"`
	FailedRequests     int64     `json:"failed_requests"`
	AverageLatency     int64     `json:"average_latency"`
	LastRequest        time.Time `json:"last_request"`
}

// IsActive checks if the node is currently active and accepting requests.
//
// Returns:
//   - bool: true if node status is NodeStatusActive
//
// Example:
//
//	for _, node := range nodes {
//	    if node.IsActive() {
//	        fmt.Printf("Active node: %s\n", node.Name)
//	    }
//	}
func (n *NodeInfo) IsActive() bool {
	return n.Status == NodeStatusActive
}

// SupportsTask checks if the node has the capability to execute a specific task.
//
// This method uses a cache to avoid repeatedly scanning the capabilities list.
// The cache is lazily initialized and rebuilt as needed.
//
// Parameters:
//   - task: The task name to check (e.g., "text_embedding", "face_detection")
//
// Returns:
//   - bool: true if the node supports the task
//
// Role in project: Essential for task-aware load balancing and ensuring requests
// are only sent to capable nodes.
//
// Example:
//
//	if node.SupportsTask("text_embedding") {
//	    // Send embedding request to this node
//	    result, err := sendRequest(node, embeddingReq)
//	}
func (n *NodeInfo) SupportsTask(task string) bool {
	n.mu.RLock()
	defer n.mu.RUnlock()

	// 使用缓存
	if n.supportedTasks == nil {
		n.rebuildSupportedTasksCache()
	}

	return n.supportedTasks[task]
}

// rebuildSupportedTasksCache 重建支持的任务缓存
func (n *NodeInfo) rebuildSupportedTasksCache() {
	n.supportedTasks = make(map[string]bool)

	// 从 Tasks 字段检查
	for _, ioTask := range n.Tasks {
		n.supportedTasks[ioTask.Name] = true
	}

	// 从 Capabilities 字段检查
	for _, capability := range n.Capabilities {
		for _, ioTask := range capability.Tasks {
			n.supportedTasks[ioTask.Name] = true
		}
	}
}

// GetConnections 获取当前连接数
func (n *NodeInfo) GetConnections() int64 {
	return atomic.LoadInt64(&n.connections)
}

// IncrementConnections 增加连接数
func (n *NodeInfo) IncrementConnections() {
	atomic.AddInt64(&n.connections, 1)
}

// DecrementConnections 减少连接数
func (n *NodeInfo) DecrementConnections() {
	atomic.AddInt64(&n.connections, -1)
}

// UpdateLoad 更新负载信息
func (n *NodeInfo) UpdateLoad(load *NodeLoad) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.Load = load
}

// UpdateStats 更新统计信息
func (n *NodeInfo) UpdateStats(stats *NodeStats) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.Stats = stats
}

// RecordRequest 记录请求
func (n *NodeInfo) RecordRequest(success bool, latency time.Duration) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.Stats == nil {
		n.Stats = &NodeStats{}
	}

	n.Stats.TotalRequests++
	n.Stats.LastRequest = time.Now()

	if success {
		n.Stats.SuccessfulRequests++
	} else {
		n.Stats.FailedRequests++
	}

	// 更新平均延迟
	if n.Stats.AverageLatency == 0 {
		n.Stats.AverageLatency = int64(latency)
	} else {
		// 指数移动平均
		alpha := 0.1
		n.Stats.AverageLatency = int64(float64(n.Stats.AverageLatency)*(1-alpha) + float64(latency)*alpha)
	}
}

// GetErrorRate 获取错误率
func (n *NodeInfo) GetErrorRate() float64 {
	n.mu.RLock()
	defer n.mu.RUnlock()

	if n.Stats == nil || n.Stats.TotalRequests == 0 {
		return 0.0
	}

	return float64(n.Stats.FailedRequests) / float64(n.Stats.TotalRequests)
}
