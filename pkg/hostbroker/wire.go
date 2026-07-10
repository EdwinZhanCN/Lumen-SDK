package hostbroker

import (
	"github.com/edwinzhancn/lumen-sdk/pkg/discovery"
)

type healthResponse struct {
	Status string `json:"status"`
}

type nodesResponse struct {
	Nodes []*discovery.NodeInfo `json:"nodes"`
}

// ---- /v1/nodes/watch wire format ----
//
// Must stay byte-compatible with discovery.BrokerResolver's parsing
// (pkg/discovery/broker_resolver.go): snapshot/added/removed messages with
// node_id, address, tasks, and txt fields. This intentionally duplicates
// the endpoint wire shape rather than importing another package.

type wsNodeInfo struct {
	NodeID  string            `json:"node_id"`
	Address string            `json:"address"`
	Tasks   []string          `json:"tasks,omitempty"`
	Txt     map[string]string `json:"txt,omitempty"`
}

type wsNodeEvent struct {
	Type   string       `json:"type"` // "snapshot", "added", "removed"
	Nodes  []wsNodeInfo `json:"nodes,omitempty"`
	Node   *wsNodeInfo  `json:"node,omitempty"`
	NodeID string       `json:"node_id,omitempty"`
}

func wsNodeInfoFrom(n *discovery.NodeInfo) wsNodeInfo {
	tasks := make([]string, 0, len(n.Tasks))
	for _, t := range n.Tasks {
		if t != nil && t.Name != "" {
			tasks = append(tasks, t.Name)
		}
	}
	var txt map[string]string
	if n.Version != "" || n.Runtime != "" {
		txt = make(map[string]string, 2)
		if n.Version != "" {
			txt["v"] = n.Version
		}
		if n.Runtime != "" {
			txt["runtime"] = n.Runtime
		}
	}
	return wsNodeInfo{
		NodeID:  n.ID,
		Address: n.Address,
		Tasks:   tasks,
		Txt:     txt,
	}
}

func nodeSnapshotMsg(nodes []*discovery.NodeInfo) wsNodeEvent {
	infos := make([]wsNodeInfo, 0, len(nodes))
	for _, n := range nodes {
		if n == nil || !n.IsActive() {
			continue
		}
		infos = append(infos, wsNodeInfoFrom(n))
	}
	return wsNodeEvent{Type: "snapshot", Nodes: infos}
}

func nodeAddedMsg(n *discovery.NodeInfo) wsNodeEvent {
	info := wsNodeInfoFrom(n)
	return wsNodeEvent{Type: "added", Node: &info}
}

func nodeRemovedMsg(nodeID string) wsNodeEvent {
	return wsNodeEvent{Type: "removed", NodeID: nodeID}
}
