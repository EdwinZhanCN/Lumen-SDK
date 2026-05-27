// Package discovery defines the unified node discovery abstraction.
//
// All discovery backends (mDNS, Gateway push, manual) implement the NodeResolver
// interface. Consumers (like the connection Pool) receive a stream of NodeEvent
// values and react by dialing or closing gRPC connections.
//
// There is no caching, TTL, health checking, or polling at this layer.
// Discovery is purely an event stream: it says who appeared and who disappeared.
// Connection health is the responsibility of the gRPC Pool.
package discovery

import "context"

// NodeEventType describes what changed.
type NodeEventType int

const (
	NodeAdded   NodeEventType = iota // a new node became reachable
	NodeRemoved                      // a node disappeared
)

// NodeEvent carries a single node change notification.
type NodeEvent struct {
	Type    NodeEventType
	NodeID  string   // stable unique identifier for this node
	Address string   // "host:port" suitable for grpc.Dial
	Tasks   []string // task names this node supports (e.g. "semantic_image_embed", "ocr")
}

// NodeResolver is the single discovery abstraction consumed by the gRPC Pool.
//
// Implementations:
//   - MDNSResolver: watches zeroconf mDNS AddService/RemoveService events.
//   - PushResolver: subscribes to a Gateway WebSocket for node events.
type NodeResolver interface {
	// Watch returns a channel that emits NodeEvent values as nodes come and go.
	// The channel is closed when ctx is cancelled or the backend stops.
	// Implementations must send a full snapshot of current nodes as a batch of
	// NodeAdded events before sending incremental changes.
	Watch(ctx context.Context) (<-chan NodeEvent, error)
}
