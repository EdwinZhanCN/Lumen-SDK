package discovery

import (
	"net"
	"strconv"
	"strings"
	"time"
)

const DefaultDeploymentID = "local"

// NodeIdentity is the stable operational identity of a Lumen node.
//
// DeploymentID plays the same scoping role as a Matter fabric for this refactor:
// the same NodeID may exist in another deployment without colliding locally.
type NodeIdentity struct {
	DeploymentID string `json:"deployment_id"`
	NodeID       string `json:"node_id"`
}

func NewNodeIdentity(deploymentID, nodeID string) NodeIdentity {
	deploymentID = strings.TrimSpace(deploymentID)
	if deploymentID == "" {
		deploymentID = DefaultDeploymentID
	}
	return NodeIdentity{
		DeploymentID: deploymentID,
		NodeID:       strings.TrimSpace(nodeID),
	}
}

func ParseNodeIdentity(instanceName, defaultDeploymentID string) NodeIdentity {
	instanceName = strings.TrimSpace(strings.TrimSuffix(instanceName, "."))
	defaultDeploymentID = strings.TrimSpace(defaultDeploymentID)
	if defaultDeploymentID == "" {
		defaultDeploymentID = DefaultDeploymentID
	}
	if instanceName == "" {
		return NewNodeIdentity(defaultDeploymentID, "")
	}

	prefix := defaultDeploymentID + "-"
	if strings.HasPrefix(instanceName, prefix) && len(instanceName) > len(prefix) {
		return NewNodeIdentity(defaultDeploymentID, strings.TrimPrefix(instanceName, prefix))
	}
	return NewNodeIdentity(defaultDeploymentID, instanceName)
}

func (i NodeIdentity) Normalized() NodeIdentity {
	return NewNodeIdentity(i.DeploymentID, i.NodeID)
}

func (i NodeIdentity) Key() string {
	i = i.Normalized()
	if i.NodeID == "" {
		return i.DeploymentID
	}
	return i.DeploymentID + "-" + i.NodeID
}

func (i NodeIdentity) IsZero() bool {
	return strings.TrimSpace(i.NodeID) == ""
}

// ResolvedNode is an operational discovery result. It is address information,
// not a liveness proof.
type ResolvedNode struct {
	Identity     NodeIdentity      `json:"identity"`
	InstanceName string            `json:"instance_name"`
	HostName     string            `json:"host_name"`
	Addresses    []string          `json:"addresses"`
	Port         int               `json:"port"`
	Txt          map[string]string `json:"txt,omitempty"`
	ExpiresAt    time.Time         `json:"expires_at,omitempty"`
}

func (n ResolvedNode) Normalized() ResolvedNode {
	n.Identity = n.Identity.Normalized()
	n.Addresses = normalizeAddresses(n.Addresses)
	if n.Txt == nil {
		n.Txt = map[string]string{}
	}
	return n
}

func (n ResolvedNode) Key() string {
	return n.Identity.Key()
}

func (n ResolvedNode) CandidateEndpoints() []string {
	n = n.Normalized()
	if n.Port <= 0 {
		return nil
	}
	out := make([]string, 0, len(n.Addresses))
	for _, addr := range n.Addresses {
		out = append(out, net.JoinHostPort(addr, strconv.Itoa(n.Port)))
	}
	return out
}

func (n ResolvedNode) Endpoint() string {
	endpoints := n.CandidateEndpoints()
	if len(endpoints) == 0 {
		return ""
	}
	return endpoints[0]
}

func (n ResolvedNode) CapHash() string {
	return n.Txt["cap_hash"]
}

func (n ResolvedNode) Version() string {
	return n.Txt["v"]
}

func (n ResolvedNode) Runtime() string {
	return n.Txt["runtime"]
}

func (n ResolvedNode) HintTasks() []string {
	return splitCSV(n.Txt["tasks"])
}

// NodeAvailability describes operational-session availability. It is more
// precise than NodeStatus, which is kept for public compatibility.
type NodeAvailability string

const (
	NodeAvailabilityUnknown       NodeAvailability = "unknown"
	NodeAvailabilityResolving     NodeAvailability = "resolving"
	NodeAvailabilityConnecting    NodeAvailability = "connecting"
	NodeAvailabilityReady         NodeAvailability = "ready"
	NodeAvailabilityDegraded      NodeAvailability = "degraded"
	NodeAvailabilityRediscovering NodeAvailability = "rediscovering"
	NodeAvailabilityUnavailable   NodeAvailability = "unavailable"
)

func (a NodeAvailability) NodeStatus() NodeStatus {
	switch a {
	case NodeAvailabilityReady:
		return NodeStatusActive
	case NodeAvailabilityConnecting, NodeAvailabilityResolving:
		return NodeStatusStarting
	case NodeAvailabilityDegraded, NodeAvailabilityRediscovering, NodeAvailabilityUnavailable:
		return NodeStatusError
	default:
		return NodeStatusUnknown
	}
}

func splitCSV(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		out = append(out, part)
	}
	return out
}

func normalizeAddresses(addresses []string) []string {
	seen := make(map[string]struct{}, len(addresses))
	out := make([]string, 0, len(addresses))
	for _, addr := range addresses {
		addr = strings.TrimSpace(strings.Trim(addr, "[]"))
		if addr == "" {
			continue
		}
		if _, ok := seen[addr]; ok {
			continue
		}
		seen[addr] = struct{}{}
		out = append(out, addr)
	}
	return out
}
