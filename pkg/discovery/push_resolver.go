package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/gorilla/websocket"
)

// PushResolver subscribes to a Gateway WebSocket endpoint for node change
// events. It implements the NodeResolver interface.
type PushResolver struct {
	hubURL string
	logger *zap.Logger
}

// NewPushResolver creates a Gateway-push-based resolver.
// hubURL is the base URL of the Gateway (e.g. "http://localhost:5866").
// The resolver connects to hubURL + "/v1/nodes/watch" via WebSocket.
func NewPushResolver(hubURL string, logger *zap.Logger) *PushResolver {
	return &PushResolver{
		hubURL: hubURL,
		logger: ensureLogger(logger),
	}
}

// Watch connects to the Gateway WebSocket and emits node events.
// On disconnect it reconnects with exponential backoff.
func (r *PushResolver) Watch(ctx context.Context) (<-chan NodeEvent, error) {
	wsURL := wsScheme(r.hubURL) + "://" + wsHost(r.hubURL) + "/v1/nodes/watch"

	ch := make(chan NodeEvent, 32)

	go func() {
		defer close(ch)

		backoff := 1 * time.Second
		const maxBackoff = 30 * time.Second

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			if err := r.connect(ctx, wsURL, ch); err != nil {
				r.logger.Warn("push resolver disconnected, reconnecting",
					zap.String("url", wsURL),
					zap.Error(err),
					zap.Duration("backoff", backoff),
				)
			} else {
				backoff = 1 * time.Second // reset on clean disconnect
			}

			// Exponential backoff with jitter.
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}()

	return ch, nil
}

func (r *PushResolver) connect(ctx context.Context, wsURL string, ch chan<- NodeEvent) error {
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return fmt.Errorf("dial ws: %w", err)
	}
	defer conn.Close()

	r.logger.Info("push resolver connected", zap.String("url", wsURL))

	// Ping handler to keep the connection alive.
	conn.SetPingHandler(func(appData string) error {
		return conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(5*time.Second))
	})

	// Read pump.
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("read ws: %w", err)
		}

		events, err := parseNodeEvents(raw)
		if err != nil {
			r.logger.Warn("failed to parse node event", zap.Error(err))
			continue
		}

		for _, ev := range events {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case ch <- ev:
			}
		}
	}
}

// pushNodeEvent is the JSON format received from the Gateway WebSocket.
type pushNodeEvent struct {
	Type   string     `json:"type"` // "snapshot", "added", "removed"
	Nodes  []pushNode `json:"nodes,omitempty"`
	Node   *pushNode  `json:"node,omitempty"`
	NodeID string     `json:"node_id,omitempty"`
}

type pushNode struct {
	NodeID  string   `json:"node_id"`
	Address string   `json:"address"`
	Tasks   []string `json:"tasks,omitempty"`
}

func parseNodeEvents(raw []byte) ([]NodeEvent, error) {
	var msg pushNodeEvent
	if err := json.Unmarshal(raw, &msg); err != nil {
		return nil, fmt.Errorf("unmarshal node event: %w", err)
	}

	switch msg.Type {
	case "snapshot":
		events := make([]NodeEvent, 0, len(msg.Nodes))
		for _, n := range msg.Nodes {
			events = append(events, NodeEvent{
				Type:    NodeAdded,
				NodeID:  n.NodeID,
				Address: n.Address,
				Tasks:   n.Tasks,
			})
		}
		return events, nil

	case "added":
		if msg.Node == nil {
			return nil, fmt.Errorf("added event missing node")
		}
		return []NodeEvent{{
			Type:    NodeAdded,
			NodeID:  msg.Node.NodeID,
			Address: msg.Node.Address,
			Tasks:   msg.Node.Tasks,
		}}, nil

	case "removed":
		if msg.NodeID == "" {
			return nil, fmt.Errorf("removed event missing node_id")
		}
		return []NodeEvent{{
			Type:   NodeRemoved,
			NodeID: msg.NodeID,
		}}, nil

	default:
		return nil, fmt.Errorf("unknown event type: %s", msg.Type)
	}
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
