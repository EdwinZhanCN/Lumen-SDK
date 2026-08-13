package discovery

import (
	"context"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// StaticResolver resolves a fixed list of node endpoints ("host:port") without
// any dynamic discovery. It implements the NodeResolver interface.
//
// Static entries are address facts, not liveness claims: the gRPC pool owns
// connection health, so an unreachable static node simply stays in a
// connecting/rediscovering state until it comes up. Static nodes are never
// expired.
type StaticResolver struct {
	endpoints    []string
	deploymentID string
	logger       *zap.Logger
	statusMu     sync.RWMutex
	status       ResolverStatus
}

// NewStaticResolver creates a resolver for a fixed endpoint list. Invalid
// entries (not "host:port") are skipped with a warning.
func NewStaticResolver(endpoints []string, deploymentID string, logger *zap.Logger) *StaticResolver {
	if deploymentID == "" {
		deploymentID = DefaultDeploymentID
	}
	return &StaticResolver{
		endpoints:    endpoints,
		deploymentID: deploymentID,
		logger:       ensureLogger(logger),
		status:       ResolverStatus{Source: "static", State: BackendStarting},
	}
}

func (r *StaticResolver) SourceName() string { return "static" }

func (r *StaticResolver) ResolverStatuses() []ResolverStatus {
	r.statusMu.RLock()
	defer r.statusMu.RUnlock()
	return []ResolverStatus{r.status}
}

// Watch emits one NodeDiscovered event per valid endpoint, then holds the
// channel open until ctx is cancelled. The resolver layer retains resolved
// entries, so a single emission is sufficient.
func (r *StaticResolver) Watch(ctx context.Context) (<-chan NodeEvent, error) {
	observedAt := time.Now().UTC()
	events := make([]NodeEvent, 0, len(r.endpoints))
	for _, endpoint := range r.endpoints {
		endpoint = strings.TrimSpace(endpoint)
		if endpoint == "" {
			continue
		}
		resolved, ok := r.resolveEndpoint(endpoint)
		if !ok {
			r.logger.Warn("skipping invalid static node endpoint", zap.String("endpoint", endpoint))
			continue
		}
		resolved.Source = r.SourceName()
		resolved.Sources = []string{r.SourceName()}
		resolved.LastObserved = observedAt
		r.logger.Info("static node resolved",
			zap.String("id", resolved.Key()),
			zap.Strings("addresses", resolved.CandidateEndpoints()),
		)
		events = append(events, eventFromResolved(NodeDiscovered, resolved))
	}
	r.statusMu.Lock()
	r.status.State = BackendHealthy
	r.status.LastScanStartedAt = observedAt
	r.status.LastScanCompletedAt = observedAt
	r.status.LastScanSucceededAt = observedAt
	r.status.LastOutcome = ScanOutcomeSuccess
	r.status.MatchedCount = len(events)
	r.status.RejectedCount = len(r.endpoints) - len(events)
	r.status.ConsecutiveFailures = 0
	r.status.LastErrorCode = ErrorCodeNone
	r.statusMu.Unlock()

	ch := make(chan NodeEvent, len(events))
	go func() {
		defer close(ch)
		for _, ev := range events {
			select {
			case ch <- ev:
			case <-ctx.Done():
				return
			}
		}
		<-ctx.Done()
	}()
	return ch, nil
}

func (r *StaticResolver) resolveEndpoint(endpoint string) (ResolvedNode, bool) {
	host, portString, err := net.SplitHostPort(endpoint)
	if err != nil {
		return ResolvedNode{}, false
	}
	port, err := strconv.Atoi(portString)
	if err != nil || port <= 0 || port > 65535 || strings.TrimSpace(host) == "" {
		return ResolvedNode{}, false
	}

	// The endpoint itself is the stable identity: static nodes have no
	// advertised instance name.
	identity := NewNodeIdentity(r.deploymentID, "static-"+endpoint)
	return ResolvedNode{
		Identity:     identity,
		InstanceName: identity.NodeID,
		Addresses:    []string{host},
		Port:         port,
	}.Normalized(), true
}
