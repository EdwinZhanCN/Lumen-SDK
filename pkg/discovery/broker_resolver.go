package discovery

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/gorilla/websocket"
)

// BrokerResolver subscribes to a Lumen Host Broker (or legacy Gateway)
// WebSocket endpoint for node change events. It implements the NodeResolver
// interface.
//
// BrokerResolver was previously named PushResolver; that name is kept as a
// deprecated alias (see push_resolver.go) so existing callers keep compiling.
// There is exactly one implementation behind both names.
type BrokerResolver struct {
	brokerURL    string
	deploymentID string
	logger       *zap.Logger
}

// NewBrokerResolver creates a Broker-push-based resolver.
// brokerURL is the base URL of the Broker (e.g. "http://localhost:5866").
// The resolver connects to brokerURL + "/v1/nodes/watch" via WebSocket.
func NewBrokerResolver(brokerURL string, logger *zap.Logger) *BrokerResolver {
	return NewBrokerResolverWithDeployment(brokerURL, DefaultDeploymentID, logger)
}

// NewBrokerResolverWithDeployment creates a Broker-push-based resolver
// scoped to a specific deployment ID.
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

// Watch connects to the Broker WebSocket and emits node events.
// On disconnect it reconnects with exponential backoff.
func (r *BrokerResolver) Watch(ctx context.Context) (<-chan NodeEvent, error) {
	wsURL := wsScheme(r.brokerURL) + "://" + wsHost(r.brokerURL) + "/v1/nodes/watch"

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
				r.logger.Warn("broker resolver disconnected, reconnecting",
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

func (r *BrokerResolver) connect(ctx context.Context, wsURL string, ch chan<- NodeEvent) error {
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return fmt.Errorf("dial ws: %w", err)
	}
	defer conn.Close()

	r.logger.Info("broker resolver connected", zap.String("url", wsURL))

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

		events, err := parseNodeEventsWithDeployment(raw, r.deploymentID)
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
