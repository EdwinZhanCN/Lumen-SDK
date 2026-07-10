package discovery

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"

	"go.uber.org/zap"
)

// PushResolver is a deprecated alias for BrokerResolver, kept so existing
// callers built against the earlier "Gateway push" name keep compiling.
// There is exactly one implementation (broker_resolver.go); no logic lives
// on this name.
//
// Deprecated: use BrokerResolver.
type PushResolver = BrokerResolver

// NewPushResolver is a deprecated alias for NewBrokerResolver.
//
// Deprecated: use NewBrokerResolver.
func NewPushResolver(hubURL string, logger *zap.Logger) *BrokerResolver {
	return NewBrokerResolver(hubURL, logger)
}

// NewPushResolverWithDeployment is a deprecated alias for
// NewBrokerResolverWithDeployment.
//
// Deprecated: use NewBrokerResolverWithDeployment.
func NewPushResolverWithDeployment(hubURL, deploymentID string, logger *zap.Logger) *BrokerResolver {
	return NewBrokerResolverWithDeployment(hubURL, deploymentID, logger)
}

// pushNodeEvent is the wire JSON format received from the Broker WebSocket.
// The field name predates the Broker rename; kept as-is since Milestone 3
// (not this change) is where the wire protocol itself gets formalized.
type pushNodeEvent struct {
	Type   string     `json:"type"` // "snapshot", "added", "removed"
	Nodes  []pushNode `json:"nodes,omitempty"`
	Node   *pushNode  `json:"node,omitempty"`
	NodeID string     `json:"node_id,omitempty"`
}

type pushNode struct {
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
	var msg pushNodeEvent
	if err := json.Unmarshal(raw, &msg); err != nil {
		return nil, fmt.Errorf("unmarshal node event: %w", err)
	}

	switch msg.Type {
	case "snapshot":
		events := make([]NodeEvent, 0, len(msg.Nodes))
		for _, n := range msg.Nodes {
			events = append(events, nodeEventFromPush(NodeDiscovered, n, deploymentID, false))
		}
		return events, nil

	case "added":
		if msg.Node == nil {
			return nil, fmt.Errorf("added event missing node")
		}
		return []NodeEvent{nodeEventFromPush(NodeDiscovered, *msg.Node, deploymentID, false)}, nil

	case "removed":
		if msg.NodeID == "" {
			return nil, fmt.Errorf("removed event missing node_id")
		}
		return []NodeEvent{nodeEventFromPush(NodeExpired, pushNode{NodeID: msg.NodeID}, deploymentID, true)}, nil

	default:
		return nil, fmt.Errorf("unknown event type: %s", msg.Type)
	}
}

func nodeEventFromPush(eventType NodeEventType, n pushNode, defaultDeploymentID string, explicitRemove bool) NodeEvent {
	deploymentID := n.DeploymentID
	if deploymentID == "" {
		deploymentID = defaultDeploymentID
	}
	identity := ParseNodeIdentity(n.NodeID, deploymentID)
	addresses, port := pushAddresses(n)
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

func pushAddresses(n pushNode) ([]string, int) {
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

// wsScheme returns "ws" or "wss" based on the hub URL scheme.
func wsScheme(hubURL string) string {
	if len(hubURL) >= 5 && hubURL[:5] == "https" {
		return "wss"
	}
	return "ws"
}

// wsHost strips the scheme prefix from the hub URL to get the host:port.
func wsHost(hubURL string) string {
	if len(hubURL) >= 7 && hubURL[:7] == "http://" {
		return hubURL[7:]
	}
	if len(hubURL) >= 8 && hubURL[:8] == "https://" {
		return hubURL[8:]
	}
	return hubURL
}
