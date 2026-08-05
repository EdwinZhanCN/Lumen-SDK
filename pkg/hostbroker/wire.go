package hostbroker

import "github.com/edwinzhancn/lumen-sdk/pkg/discovery"

type healthResponse struct {
	Status string `json:"status"`
}

type nodesResponse struct {
	Nodes []*discovery.NodeInfo `json:"nodes"`
}

// Host Broker is an experimental address-discovery bridge. It forwards only
// identity, endpoint, and descriptive TXT metadata; downstream SDK clients
// always fetch task and protocol capabilities in-band from the node.
type wsNodeInfo struct {
	NodeID  string            `json:"node_id"`
	Address string            `json:"address"`
	Txt     map[string]string `json:"txt,omitempty"`
}

type wsNodeEvent struct {
	Type  string       `json:"type"`
	Nodes []wsNodeInfo `json:"nodes"`
}

func wsNodeInfoFrom(node *discovery.NodeInfo) wsNodeInfo {
	var txt map[string]string
	if node.Version != "" || node.Runtime != "" {
		txt = make(map[string]string, 2)
		if node.Version != "" {
			txt["v"] = node.Version
		}
		if node.Runtime != "" {
			txt["runtime"] = node.Runtime
		}
	}
	return wsNodeInfo{NodeID: node.ID, Address: node.Address, Txt: txt}
}

func nodeSnapshotMsg(nodes []*discovery.NodeInfo) wsNodeEvent {
	infos := make([]wsNodeInfo, 0, len(nodes))
	for _, node := range nodes {
		if node != nil && node.ID != "" && node.Address != "" {
			infos = append(infos, wsNodeInfoFrom(node))
		}
	}
	return wsNodeEvent{Type: "snapshot", Nodes: infos}
}
