// Package discovery defines the unified operational discovery abstraction.
//
// All discovery backends (mDNS, Broker push, manual) implement the
// NodeResolver interface. Consumers receive a stream of NodeEvent values that
// describe address-resolution facts. Discovery does not prove node liveness;
// connection health belongs to the operational session / gRPC pool layer.
package discovery

import "context"

// NodeEventType describes what changed.
type NodeEventType int

const (
	NodeDiscovered    NodeEventType = iota // a service instance or resolved address was discovered
	NodeExpired                            // DNS-SD TTL expired or a push backend explicitly revoked the node
	NodeResolveFailed                      // address resolution failed; this is not a liveness verdict

	// Deprecated: use NodeDiscovered. Kept as a source-compatible alias for
	// callers built against the previous resolver event names.
	NodeAdded = NodeDiscovered
	// Deprecated: use NodeExpired. Kept as a source-compatible alias for
	// callers built against the previous resolver event names.
	NodeRemoved = NodeExpired
)

// NodeEvent carries a single operational discovery notification.
type NodeEvent struct {
	Type     NodeEventType
	Identity NodeIdentity
	Resolved ResolvedNode

	NodeID    string            // stable unique identifier; deprecated alias for Identity.Key()
	Address   string            // first "host:port" candidate; deprecated convenience field
	Addresses []string          // candidate "host:port" values suitable for grpc.Dial
	Tasks     []string          // lightweight task hints from TXT / push payload
	Txt       map[string]string // TXT key/value records
	Err       error             // set when Type is NodeResolveFailed

	// ExplicitRemove is true when the producer knows the node should be
	// removed, such as a Gateway "removed" event. mDNS TTL expiry should leave
	// this false because stale DNS-SD records are not liveness proof.
	ExplicitRemove bool
}

// NodeResolver is the single discovery abstraction consumed by the gRPC Pool.
//
// Implementations:
//   - MDNSResolver: watches and resolves zeroconf mDNS service records.
//   - BrokerResolver: subscribes to a Broker WebSocket for node events
//     (formerly PushResolver, kept as a deprecated alias).
type NodeResolver interface {
	// Watch returns a channel that emits operational discovery events.
	// The channel is closed when ctx is cancelled or the backend stops.
	Watch(ctx context.Context) (<-chan NodeEvent, error)
}
