package discovery

import (
	"sync"
	"sync/atomic"
	"time"

	pb "github.com/edwinzhancn/lumen-sdk/proto"
)

// NodeInfo represents a discovered ML inference node with its capabilities and status.
type NodeInfo struct {
	ID           string                 `json:"id"`
	Address      string                 `json:"address"`
	Status       NodeStatus             `json:"status"`
	Availability NodeAvailability       `json:"availability,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	Capabilities []*pb.Capability       `json:"capabilities,omitempty"`
	Version      string                 `json:"version"`
	Runtime      string                 `json:"runtime"`
	Models       []*ModelInfo           `json:"models,omitempty"`
	LastSeen     time.Time              `json:"last_seen"`
	Tasks        []*pb.IOTask           `json:"tasks,omitempty"`

	connections    int64           `json:"-"`
	supportedTasks map[string]bool `json:"-"`
	mu             sync.RWMutex    `json:"-"`
}

type ModelInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version"`
	Runtime string `json:"runtime"`
}

type NodeStatus string

const (
	NodeStatusUnknown  NodeStatus = "unknown"
	NodeStatusStarting NodeStatus = "starting"
	NodeStatusActive   NodeStatus = "active"
	NodeStatusError    NodeStatus = "error"
)

func (n *NodeInfo) IsActive() bool {
	return n.Status == NodeStatusActive
}

func (n *NodeInfo) SupportsTask(task string) bool {
	n.mu.RLock()
	cache := n.supportedTasks
	if cache != nil {
		supported := cache[task]
		n.mu.RUnlock()
		return supported
	}
	n.mu.RUnlock()

	n.mu.Lock()
	defer n.mu.Unlock()
	if n.supportedTasks == nil {
		n.rebuildSupportedTasksCacheLocked()
	}
	return n.supportedTasks[task]
}

func (n *NodeInfo) SupportsServiceTask(service, task string) bool {
	if service == "" {
		return n.SupportsTask(task)
	}
	n.mu.RLock()
	defer n.mu.RUnlock()
	for _, capability := range n.Capabilities {
		if capability.GetServiceName() != service {
			continue
		}
		for _, ioTask := range capability.GetTasks() {
			if ioTask.GetName() == task {
				return true
			}
		}
	}
	return false
}

func (n *NodeInfo) MatchingServices(task string) []string {
	n.mu.RLock()
	defer n.mu.RUnlock()
	seen := make(map[string]bool)
	var services []string
	for _, capability := range n.Capabilities {
		for _, ioTask := range capability.GetTasks() {
			if ioTask.GetName() == task {
				service := capability.GetServiceName()
				if service != "" && !seen[service] {
					seen[service] = true
					services = append(services, service)
				}
			}
		}
	}
	return services
}

func (n *NodeInfo) rebuildSupportedTasksCacheLocked() {
	n.supportedTasks = make(map[string]bool)

	for _, ioTask := range n.Tasks {
		n.supportedTasks[ioTask.Name] = true
	}

	for _, capability := range n.Capabilities {
		for _, ioTask := range capability.Tasks {
			n.supportedTasks[ioTask.Name] = true
		}
	}
}

func (n *NodeInfo) InvalidateTaskCache() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.supportedTasks = nil
}

func (n *NodeInfo) GetConnections() int64 {
	return atomic.LoadInt64(&n.connections)
}

func (n *NodeInfo) IncrementConnections() {
	atomic.AddInt64(&n.connections, 1)
}

func (n *NodeInfo) DecrementConnections() {
	atomic.AddInt64(&n.connections, -1)
}
