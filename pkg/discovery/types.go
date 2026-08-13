package discovery

import (
	"time"

	pb "github.com/edwinzhancn/lumen-sdk/proto"
)

// CompatibilityState is the in-band protocol verdict for a connected node.
// Discovery never sets this state; the gRPC capability exchange is authoritative.
type CompatibilityState string

const (
	CompatibilityPending      CompatibilityState = "pending"
	CompatibilityCompatible   CompatibilityState = "compatible"
	CompatibilityIncompatible CompatibilityState = "incompatible"
)

// NodeInfo is an immutable snapshot of one discovered node's operational
// session. Availability and Compatibility are intentionally orthogonal: a
// transport can be ready while the node is protocol-incompatible.
type NodeInfo struct {
	ID            string                 `json:"id"`
	Address       string                 `json:"address"`
	Sources       []string               `json:"sources,omitempty"`
	LastObserved  time.Time              `json:"last_observed_at,omitempty"`
	Availability  NodeAvailability       `json:"availability"`
	Compatibility CompatibilityState     `json:"compatibility"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	Capabilities  []*pb.Capability       `json:"capabilities,omitempty"`
	Version       string                 `json:"version,omitempty"`
	Runtime       string                 `json:"runtime,omitempty"`
	Models        []*ModelInfo           `json:"models,omitempty"`
	UpdatedAt     time.Time              `json:"updated_at"`

	// IncompatibleReason explains why the node is excluded from routing.
	IncompatibleReason string `json:"incompatible_reason,omitempty"`
}

type ModelInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version"`
	Runtime string `json:"runtime"`
}

func (n *NodeInfo) IsActive() bool {
	return n != nil && n.Availability == NodeAvailabilityReady && n.Compatibility == CompatibilityCompatible
}

func (n *NodeInfo) SupportsTask(task string) bool {
	if n == nil || task == "" {
		return false
	}
	for _, capability := range n.Capabilities {
		for _, ioTask := range capability.GetTasks() {
			if ioTask.GetName() == task {
				return true
			}
		}
	}
	return false
}

func (n *NodeInfo) SupportsServiceTask(service, task string) bool {
	if n == nil || task == "" {
		return false
	}
	if service == "" {
		return n.SupportsTask(task)
	}
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
	if n == nil || task == "" {
		return nil
	}
	seen := make(map[string]bool)
	var services []string
	for _, capability := range n.Capabilities {
		for _, ioTask := range capability.GetTasks() {
			if ioTask.GetName() != task {
				continue
			}
			service := capability.GetServiceName()
			if service != "" && !seen[service] {
				seen[service] = true
				services = append(services, service)
			}
		}
	}
	return services
}
