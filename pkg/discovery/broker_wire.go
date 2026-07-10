package discovery

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
)

// brokerNodeEvent is the wire JSON format received from the Broker WebSocket.
type brokerNodeEvent struct {
	Type   string       `json:"type"` // "snapshot", "added", "removed"
	Nodes  []brokerNode `json:"nodes,omitempty"`
	Node   *brokerNode  `json:"node,omitempty"`
	NodeID string       `json:"node_id,omitempty"`
}

type brokerNode struct {
	NodeID       string            `json:"node_id"`
	DeploymentID string            `json:"deployment_id,omitempty"`
	Address      string            `json:"address"`
	Addresses    []string          `json:"addresses,omitempty"`
	Port         int               `json:"port,omitempty"`
	Tasks        []string          `json:"tasks,omitempty"`
	Txt          map[string]string `json:"txt,omitempty"`
}

func parseNodeEvents(raw []byte) ([]NodeEvent, error) {
	return parseNodeEventsWithDeployment(raw, DefaultDeploymentID)
}

func parseNodeEventsWithDeployment(raw []byte, deploymentID string) ([]NodeEvent, error) {
	var msg brokerNodeEvent
	if err := json.Unmarshal(raw, &msg); err != nil {
		return nil, fmt.Errorf("unmarshal node event: %w", err)
	}

	switch msg.Type {
	case "snapshot":
		events := make([]NodeEvent, 0, len(msg.Nodes))
		for _, n := range msg.Nodes {
			events = append(events, nodeEventFromBroker(NodeDiscovered, n, deploymentID, false))
		}
		return events, nil

	case "added":
		if msg.Node == nil {
			return nil, fmt.Errorf("added event missing node")
		}
		return []NodeEvent{nodeEventFromBroker(NodeDiscovered, *msg.Node, deploymentID, false)}, nil

	case "removed":
		if msg.NodeID == "" {
			return nil, fmt.Errorf("removed event missing node_id")
		}
		return []NodeEvent{nodeEventFromBroker(NodeExpired, brokerNode{NodeID: msg.NodeID}, deploymentID, true)}, nil

	default:
		return nil, fmt.Errorf("unknown event type: %s", msg.Type)
	}
}

func nodeEventFromBroker(eventType NodeEventType, n brokerNode, defaultDeploymentID string, explicitRemove bool) NodeEvent {
	deploymentID := n.DeploymentID
	if deploymentID == "" {
		deploymentID = defaultDeploymentID
	}
	identity := ParseNodeIdentity(n.NodeID, deploymentID)
	addresses, port := brokerAddresses(n)
	txt := make(map[string]string, len(n.Txt)+1)
	for k, v := range n.Txt {
		txt[k] = v
	}
	if len(n.Tasks) > 0 {
		txt["tasks"] = joinCSV(n.Tasks)
	}
	resolved := ResolvedNode{
		Identity:     identity,
		InstanceName: identity.Key(),
		Addresses:    addresses,
		Port:         port,
		Txt:          txt,
	}.Normalized()
	ev := eventFromResolved(eventType, resolved)
	ev.ExplicitRemove = explicitRemove
	return ev
}

func brokerAddresses(n brokerNode) ([]string, int) {
	port := n.Port
	addresses := append([]string(nil), n.Addresses...)
	if n.Address != "" {
		host, parsedPort, err := splitHostPort(n.Address)
		if err == nil {
			if port == 0 {
				port = parsedPort
			}
			addresses = append(addresses, host)
		} else {
			addresses = append(addresses, n.Address)
		}
	}
	return addresses, port
}

func splitHostPort(address string) (string, int, error) {
	host, portString, err := net.SplitHostPort(address)
	if err != nil {
		return "", 0, err
	}
	port, err := strconv.Atoi(portString)
	if err != nil {
		return "", 0, err
	}
	return host, port, nil
}

func joinCSV(values []string) string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return strings.Join(out, ",")
}

// wsScheme returns "ws" or "wss" based on the Broker URL scheme.
func wsScheme(brokerURL string) string {
	if len(brokerURL) >= 5 && brokerURL[:5] == "https" {
		return "wss"
	}
	return "ws"
}

// wsHost strips the scheme prefix from the Broker URL to get the host:port.
func wsHost(brokerURL string) string {
	if len(brokerURL) >= 7 && brokerURL[:7] == "http://" {
		return brokerURL[7:]
	}
	if len(brokerURL) >= 8 && brokerURL[:8] == "https://" {
		return brokerURL[8:]
	}
	return brokerURL
}
