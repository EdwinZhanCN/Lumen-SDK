package client

import (
	pb "Lumen-SDK/proto"
	"sync"
	"sync/atomic"
	"time"
)

type NodeInfo struct {
	ID           string
	Name         string
	Address      string
	Status       NodeStatus
	Metadata     map[string]interface{}
	Capabilities []*pb.Capability
	Version      string
	Runtime      string
	Models       []*ModelInfo
	LastSeen     time.Time
	Tasks        []*pb.IOTask

	// 负载均衡相关字段
	Weight         int64           `json:"weight"`
	Load           *NodeLoad       `json:"load,omitempty"`
	Stats          *NodeStats      `json:"stats,omitempty"`
	connections    int64           `json:"-"` // 当前连接数
	supportedTasks map[string]bool `json:"-"` // 支持的任务缓存
	mu             sync.RWMutex    `json:"-"` // 读写锁
}

type ModelInfo struct {
	ID      string
	Name    string
	Version string
	Runtime string
}

type NodeStatus string

const (
	NodeStatusUnknown  NodeStatus = "unknown"
	NodeStatusStarting NodeStatus = "starting"
	NodeStatusActive   NodeStatus = "active"
	NodeStatusError    NodeStatus = "error"
)

// NodeLoad 节点负载信息
type NodeLoad struct {
	CPU    float64 `json:"cpu"`    // CPU 使用率 0-1
	Memory float64 `json:"memory"` // 内存使用率 0-1
	GPU    float64 `json:"gpu"`    // GPU 使用率 0-1
	Disk   float64 `json:"disk"`   // 磁盘使用率 0-1
}

// NodeStats 节点统计信息
type NodeStats struct {
	TotalRequests      int64     `json:"total_requests"`
	SuccessfulRequests int64     `json:"successful_requests"`
	FailedRequests     int64     `json:"failed_requests"`
	AverageLatency     int64     `json:"average_latency_ms"`
	LastRequest        time.Time `json:"last_request"`
}

// IsActive 检查节点是否活跃
func (n *NodeInfo) IsActive() bool {
	return n.Status == NodeStatusActive
}

// SupportsTask 检查节点是否支持指定任务
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
