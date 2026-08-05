package discovery

import (
	"context"
	"fmt"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// BrokerResolver subscribes to complete address snapshots from an experimental
// Lumen Host Broker. Snapshot replacement is translated into the same
// discovered/expired event stream used by every other resolver.
type BrokerResolver struct {
	brokerURL    string
	deploymentID string
	logger       *zap.Logger
}

func NewBrokerResolver(brokerURL string, logger *zap.Logger) *BrokerResolver {
	return NewBrokerResolverWithDeployment(brokerURL, DefaultDeploymentID, logger)
}

func NewBrokerResolverWithDeployment(brokerURL, deploymentID string, logger *zap.Logger) *BrokerResolver {
	if deploymentID == "" {
		deploymentID = DefaultDeploymentID
	}
	return &BrokerResolver{
		brokerURL:    brokerURL,
		deploymentID: deploymentID,
		logger:       ensureLogger(logger),
	}
}

// Watch connects to Host Broker and reconnects with bounded exponential
// backoff. The last accepted snapshot is retained across reconnects and is
// replaced by the next valid snapshot.
func (r *BrokerResolver) Watch(ctx context.Context) (<-chan NodeEvent, error) {
	wsURL, err := brokerWebSocketURL(r.brokerURL)
	if err != nil {
		return nil, err
	}

	ch := make(chan NodeEvent, 32)
	go func() {
		defer close(ch)
		known := make(map[string]ResolvedNode)
		backoff := time.Second
		const maxBackoff = 30 * time.Second

		for {
			if err := ctx.Err(); err != nil {
				return
			}
			receivedSnapshot, err := r.connect(ctx, wsURL, ch, known)
			if receivedSnapshot {
				// A healthy session should not inherit backoff accumulated by old
				// connection failures.
				backoff = time.Second
			}
			if err != nil && ctx.Err() == nil {
				r.logger.Warn("broker resolver disconnected; reconnecting",
					zap.String("url", wsURL),
					zap.Error(err),
					zap.Duration("backoff", backoff),
				)
			}

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

func (r *BrokerResolver) connect(
	ctx context.Context,
	wsURL string,
	ch chan<- NodeEvent,
	known map[string]ResolvedNode,
) (bool, error) {
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return false, fmt.Errorf("dial Broker WebSocket: %w", err)
	}
	defer conn.Close()
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-done:
		}
	}()

	r.logger.Info("broker resolver connected", zap.String("url", wsURL))
	conn.SetPingHandler(func(appData string) error {
		return conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(5*time.Second))
	})

	receivedSnapshot := false
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return receivedSnapshot, fmt.Errorf("read Broker WebSocket: %w", err)
		}
		snapshot, err := parseNodeSnapshot(raw, r.deploymentID)
		if err != nil {
			r.logger.Warn("rejected Broker snapshot", zap.Error(err))
			continue
		}
		if err := emitSnapshotReplacement(ctx, ch, known, snapshot); err != nil {
			return receivedSnapshot, err
		}
		receivedSnapshot = true
	}
}

func emitSnapshotReplacement(
	ctx context.Context,
	ch chan<- NodeEvent,
	known map[string]ResolvedNode,
	snapshot []ResolvedNode,
) error {
	current := make(map[string]ResolvedNode, len(snapshot))
	for _, node := range snapshot {
		current[node.Key()] = node
	}

	for key, previous := range known {
		if _, exists := current[key]; !exists {
			if err := emitNodeEvent(ctx, ch, eventFromResolved(NodeExpired, previous)); err != nil {
				return err
			}
		}
	}
	for _, node := range snapshot {
		// Re-emitting retained identities is intentional: it replaces endpoint and
		// descriptive metadata without a second added/updated wire protocol.
		if err := emitNodeEvent(ctx, ch, eventFromResolved(NodeDiscovered, node)); err != nil {
			return err
		}
	}

	clear(known)
	for key, node := range current {
		known[key] = node
	}
	return nil
}

func emitNodeEvent(ctx context.Context, ch chan<- NodeEvent, event NodeEvent) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case ch <- event:
		return nil
	}
}
