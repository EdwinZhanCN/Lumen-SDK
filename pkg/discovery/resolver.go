// Package discovery defines the unified operational discovery abstraction.
//
// All discovery backends (mDNS, Broker push, manual) implement the
// NodeResolver interface. Consumers receive address-resolution facts only;
// connection health and protocol compatibility belong to the gRPC pool.
package discovery

import (
	"context"
	"sort"
	"time"
)

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

// BackendState is the externally observable lifecycle of one discovery
// source. It deliberately says nothing about transport or protocol health.
type BackendState string

const (
	BackendDisabled BackendState = "disabled"
	BackendStarting BackendState = "starting"
	BackendHealthy  BackendState = "healthy"
	BackendDegraded BackendState = "degraded"
)

// ScanOutcome classifies a bounded discovery scan without exposing a backend
// library's raw error text.
type ScanOutcome string

const (
	ScanOutcomeSuccess   ScanOutcome = "success"
	ScanOutcomeFailed    ScanOutcome = "failed"
	ScanOutcomeTimedOut  ScanOutcome = "timed_out"
	ScanOutcomeCancelled ScanOutcome = "cancelled"
)

const (
	ErrorCodeNone             = ""
	ErrorCodeCancelled        = "cancelled"
	ErrorCodeQueryFailed      = "query_failed"
	ErrorCodeQueryTimedOut    = "query_timed_out"
	ErrorCodeResolveFailed    = "resolve_failed"
	ErrorCodeSocketOpenFailed = "socket_open_failed"
	ErrorCodeSocketSendFailed = "socket_send_failed"
	ErrorCodeSocketReadFailed = "socket_read_failed"
	ErrorCodeWatchStartFailed = "watch_start_failed"
	ErrorCodeWatchClosed      = "watch_closed"
)

// ResolverStatus is an immutable, bounded diagnostic projection for one
// discovery source. It contains no instance identities, addresses, packets,
// TXT data, or arbitrary error strings.
type ResolverStatus struct {
	Source              string       `json:"source"`
	State               BackendState `json:"state"`
	LastScanStartedAt   time.Time    `json:"last_scan_started_at,omitempty"`
	LastScanCompletedAt time.Time    `json:"last_scan_completed_at,omitempty"`
	LastScanSucceededAt time.Time    `json:"last_scan_succeeded_at,omitempty"`
	NextScanAt          time.Time    `json:"next_scan_at,omitempty"`
	ConsecutiveFailures int          `json:"consecutive_failures"`
	LastErrorCode       string       `json:"last_error_code,omitempty"`
	MatchedCount        int          `json:"matched_count"`
	RejectedCount       int          `json:"rejected_count"`
	LastOutcome         ScanOutcome  `json:"last_outcome,omitempty"`
}

// ResolverStatusProvider is optional so existing external NodeResolver test
// doubles and implementations remain source-compatible.
type ResolverStatusProvider interface {
	ResolverStatuses() []ResolverStatus
}

// ResolveNowProvider is implemented by sources that can coalesce a request for
// an expedited refresh. Scheduled discovery remains the correctness path.
type ResolveNowProvider interface {
	ResolveNow()
}

// SourceNameProvider gives CompositeResolver a stable, bounded ownership key.
type SourceNameProvider interface {
	SourceName() string
}

// Statuses returns a defensive, deterministically ordered snapshot when the
// resolver exposes diagnostics.
func Statuses(resolver NodeResolver) []ResolverStatus {
	provider, ok := resolver.(ResolverStatusProvider)
	if !ok || provider == nil {
		return []ResolverStatus{}
	}
	statuses := append([]ResolverStatus(nil), provider.ResolverStatuses()...)
	sort.Slice(statuses, func(i, j int) bool { return statuses[i].Source < statuses[j].Source })
	return statuses
}

// RequestResolveNow asks a supporting resolver for an expedited scan.
func RequestResolveNow(resolver NodeResolver) {
	if provider, ok := resolver.(ResolveNowProvider); ok && provider != nil {
		provider.ResolveNow()
	}
}
