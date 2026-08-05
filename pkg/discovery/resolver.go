// Package discovery defines the unified operational discovery abstraction.
//
// All discovery backends (mDNS, Broker push, manual) implement the
// NodeResolver interface. Consumers receive address-resolution facts only;
// connection health and protocol compatibility belong to the gRPC pool.
package discovery

import "context"

// NodeEventType describes what changed.
type NodeEventType int

const (
	NodeDiscovered    NodeEventType = iota // a service instance or address was discovered
	NodeExpired                            // the resolver revoked or expired the address
	NodeResolveFailed                      // address resolution failed; not a liveness verdict
)

// NodeEvent carries one canonical discovery record. Resolved contains the
// identity, addresses, and discovery metadata; parallel copies are forbidden.
type NodeEvent struct {
	Type     NodeEventType
	Resolved ResolvedNode
	Err      error // set when Type is NodeResolveFailed
}

// NodeResolver is the single discovery abstraction consumed by the gRPC Pool.
type NodeResolver interface {
	Watch(ctx context.Context) (<-chan NodeEvent, error)
}
