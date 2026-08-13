package client

import (
	"time"

	"github.com/edwinzhancn/lumen-sdk/pkg/discovery"
)

// RuntimeCounts is the aggregate projection of independent discovery,
// transport, and compatibility dimensions.
type RuntimeCounts struct {
	DiscoveredNodes   int `json:"discovered_nodes"`
	ActiveNodes       int `json:"active_nodes"`
	ConnectingNodes   int `json:"connecting_nodes"`
	UnavailableNodes  int `json:"unavailable_nodes"`
	PendingNodes      int `json:"pending_nodes"`
	IncompatibleNodes int `json:"incompatible_nodes"`
}

// RuntimeSnapshot is an immutable diagnostic view assembled from the
// resolver and pool. Capability sets and nodes are defensively cloned.
type RuntimeSnapshot struct {
	CapturedAt     time.Time                  `json:"captured_at"`
	DiscoveryState discovery.BackendState     `json:"discovery_state"`
	Backends       []discovery.ResolverStatus `json:"backends"`
	Counts         RuntimeCounts              `json:"counts"`
	Nodes          []*discovery.NodeInfo      `json:"nodes"`
}

func (c *LumenClient) RuntimeSnapshot() RuntimeSnapshot {
	nodes := c.pool.NodeInfos()
	backends := discovery.Statuses(c.resolver)
	snapshot := RuntimeSnapshot{
		CapturedAt:     time.Now().UTC(),
		DiscoveryState: aggregateDiscoveryState(backends),
		Backends:       backends,
		Nodes:          discovery.CloneNodeSlice(nodes),
	}
	snapshot.Counts.DiscoveredNodes = len(snapshot.Nodes)
	for _, node := range snapshot.Nodes {
		if node == nil {
			continue
		}
		if node.IsActive() {
			snapshot.Counts.ActiveNodes++
		}
		switch node.Availability {
		case discovery.NodeAvailabilityConnecting, discovery.NodeAvailabilityDiscovered:
			snapshot.Counts.ConnectingNodes++
		case discovery.NodeAvailabilityUnavailable:
			snapshot.Counts.UnavailableNodes++
		}
		switch node.Compatibility {
		case discovery.CompatibilityPending:
			snapshot.Counts.PendingNodes++
		case discovery.CompatibilityIncompatible:
			snapshot.Counts.IncompatibleNodes++
		}
	}
	return snapshot
}

func (c *LumenClient) ResolveNow() {
	discovery.RequestResolveNow(c.resolver)
}

func aggregateDiscoveryState(statuses []discovery.ResolverStatus) discovery.BackendState {
	if len(statuses) == 0 {
		return discovery.BackendDisabled
	}
	hasHealthy := false
	hasStarting := false
	for _, status := range statuses {
		switch status.State {
		case discovery.BackendDegraded:
			return discovery.BackendDegraded
		case discovery.BackendStarting:
			hasStarting = true
		case discovery.BackendHealthy:
			hasHealthy = true
		}
	}
	if hasStarting {
		return discovery.BackendStarting
	}
	if hasHealthy {
		return discovery.BackendHealthy
	}
	return discovery.BackendDisabled
}
