package discovery

import (
	"context"
	"fmt"
	"sync"
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
	statusMu     sync.RWMutex
	status       ResolverStatus
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
		status:       ResolverStatus{Source: "broker", State: BackendStarting},
	}
}

func (r *BrokerResolver) SourceName() string { return "broker" }

func (r *BrokerResolver) ResolverStatuses() []ResolverStatus {
	r.statusMu.RLock()
	defer r.statusMu.RUnlock()
	return []ResolverStatus{r.status}
}

// Watch connects to Host Broker and reconnects with bounded exponential
// backoff. The last accepted snapshot is retained across reconnects and is
// replaced by the next valid snapshot.
func (r *BrokerResolver) Watch(ctx context.Context) (<-chan NodeEvent, error) {
	wsURL, err := brokerWebSocketURL(r.brokerURL)
	if err != nil {
		r.recordBrokerFailure(ErrorCodeWatchStartFailed)
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
				r.recordBrokerFailure(ErrorCodeWatchClosed)
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
		observedAt := time.Now().UTC()
		for index := range snapshot {
			snapshot[index].Source = r.SourceName()
			snapshot[index].Sources = []string{r.SourceName()}
			snapshot[index].LastObserved = observedAt
			snapshot[index] = snapshot[index].Normalized()
		}
		if err := emitSnapshotReplacement(ctx, ch, known, snapshot); err != nil {
			return receivedSnapshot, err
		}
		r.recordBrokerSuccess(observedAt, len(snapshot))
		receivedSnapshot = true
	}
}

func (r *BrokerResolver) recordBrokerFailure(code string) {
	now := time.Now().UTC()
	r.statusMu.Lock()
	defer r.statusMu.Unlock()
	r.status.State = BackendDegraded
	r.status.LastScanStartedAt = now
	r.status.LastScanCompletedAt = now
	r.status.LastOutcome = ScanOutcomeFailed
	r.status.LastErrorCode = code
	r.status.ConsecutiveFailures++
}

func (r *BrokerResolver) recordBrokerSuccess(at time.Time, matched int) {
	r.statusMu.Lock()
	defer r.statusMu.Unlock()
	r.status.State = BackendHealthy
	r.status.LastScanStartedAt = at
	r.status.LastScanCompletedAt = at
	r.status.LastScanSucceededAt = at
	r.status.LastOutcome = ScanOutcomeSuccess
	r.status.LastErrorCode = ErrorCodeNone
	r.status.ConsecutiveFailures = 0
	r.status.MatchedCount = clampDiagnosticCount(matched)
	r.status.RejectedCount = 0
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
