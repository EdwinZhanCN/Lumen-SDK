package discovery

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// brokerNodeEvent is a complete address snapshot received from Host Broker.
type brokerNodeEvent struct {
	Type  string       `json:"type"`
	Nodes []brokerNode `json:"nodes"`
}

type brokerNode struct {
	NodeID  string            `json:"node_id"`
	Address string            `json:"address"`
	Txt     map[string]string `json:"txt,omitempty"`
}

func parseNodeSnapshot(raw []byte, deploymentID string) ([]ResolvedNode, error) {
	var msg brokerNodeEvent
	if err := json.Unmarshal(raw, &msg); err != nil {
		return nil, fmt.Errorf("unmarshal node snapshot: %w", err)
	}
	if msg.Type != "snapshot" {
		return nil, fmt.Errorf("unsupported Broker event type %q; expected snapshot", msg.Type)
	}

	seen := make(map[string]struct{}, len(msg.Nodes))
	resolved := make([]ResolvedNode, 0, len(msg.Nodes))
	for index, node := range msg.Nodes {
		identity := ParseNodeIdentity(node.NodeID, deploymentID)
		if identity.IsZero() {
			return nil, fmt.Errorf("snapshot node %d is missing node_id", index)
		}
		host, portString, err := net.SplitHostPort(strings.TrimSpace(node.Address))
		port, portErr := strconv.Atoi(portString)
		if err != nil || portErr != nil || strings.TrimSpace(host) == "" || port <= 0 || port > 65535 {
			return nil, fmt.Errorf("snapshot node %q has invalid address %q", node.NodeID, node.Address)
		}
		if _, exists := seen[identity.Key()]; exists {
			return nil, fmt.Errorf("snapshot contains duplicate node identity %q", identity.Key())
		}
		seen[identity.Key()] = struct{}{}

		// Host Broker metadata is descriptive only. Authority-like task/protocol
		// hints are discarded before they enter the resolver state.
		txt := make(map[string]string, 2)
		if version := strings.TrimSpace(node.Txt["v"]); version != "" {
			txt["v"] = version
		}
		if runtime := strings.TrimSpace(node.Txt["runtime"]); runtime != "" {
			txt["runtime"] = runtime
		}
		resolved = append(resolved, ResolvedNode{
			Identity:     identity,
			InstanceName: identity.Key(),
			Addresses:    []string{host},
			Port:         port,
			Txt:          txt,
		}.Normalized())
	}
	return resolved, nil
}

func brokerWebSocketURL(baseURL string) (string, error) {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return "", fmt.Errorf("Broker URL is empty")
	}
	if !strings.Contains(baseURL, "://") {
		baseURL = "http://" + baseURL
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("parse Broker URL: %w", err)
	}
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	default:
		return "", fmt.Errorf("unsupported Broker URL scheme %q", u.Scheme)
	}
	if u.Host == "" {
		return "", fmt.Errorf("Broker URL has no host")
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/v1/nodes/watch"
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}
