package client

import (
	"context"
	"testing"
	"time"

	"github.com/edwinzhancn/lumen-sdk/pkg/discovery"
	"google.golang.org/grpc/connectivity"
)

type statusNodeResolver struct {
	statuses []discovery.ResolverStatus
	refresh  int
}

func (r *statusNodeResolver) Watch(ctx context.Context) (<-chan discovery.NodeEvent, error) {
	stream := make(chan discovery.NodeEvent)
	go func() {
		defer close(stream)
		<-ctx.Done()
	}()
	return stream, nil
}

func (r *statusNodeResolver) ResolverStatuses() []discovery.ResolverStatus {
	return append([]discovery.ResolverStatus(nil), r.statuses...)
}

func (r *statusNodeResolver) ResolveNow() { r.refresh++ }

func TestRuntimeSnapshotKeepsStateDimensionsIndependent(t *testing.T) {
	now := time.Now().UTC()
	resolver := &statusNodeResolver{statuses: []discovery.ResolverStatus{{
		Source: "mdns", State: discovery.BackendHealthy, MatchedCount: 3,
	}}}
	registry := &nodeRegistry{nodes: map[string]*registeredNode{
		"active": {
			identity: discovery.NewNodeIdentity("local", "active"), state: connectivity.Ready,
			compatibility: discovery.CompatibilityCompatible, sources: []string{"mdns"}, lastObserved: now,
		},
		"pending": {
			identity: discovery.NewNodeIdentity("local", "pending"), state: connectivity.Connecting,
			compatibility: discovery.CompatibilityPending, sources: []string{"mdns"}, lastObserved: now,
		},
		"incompatible": {
			identity: discovery.NewNodeIdentity("local", "incompatible"), state: connectivity.Ready,
			compatibility: discovery.CompatibilityIncompatible, sources: []string{"mdns"}, lastObserved: now,
		},
	}}
	client := &LumenClient{pool: &Pool{registry: registry}, resolver: resolver}

	snapshot := client.RuntimeSnapshot()
	if snapshot.DiscoveryState != discovery.BackendHealthy {
		t.Fatalf("discovery state = %q", snapshot.DiscoveryState)
	}
	if snapshot.Counts.DiscoveredNodes != 3 || snapshot.Counts.ActiveNodes != 1 ||
		snapshot.Counts.ConnectingNodes != 1 || snapshot.Counts.PendingNodes != 1 ||
		snapshot.Counts.IncompatibleNodes != 1 {
		t.Fatalf("unexpected counts: %+v", snapshot.Counts)
	}
	if len(snapshot.Nodes) != 3 || len(snapshot.Backends) != 1 {
		t.Fatalf("incomplete snapshot: %+v", snapshot)
	}

	snapshot.Nodes[0].Sources[0] = "mutated"
	again := client.RuntimeSnapshot()
	for _, node := range again.Nodes {
		if len(node.Sources) > 0 && node.Sources[0] == "mutated" {
			t.Fatal("runtime snapshot leaked mutable node state")
		}
	}

	client.ResolveNow()
	if resolver.refresh != 1 {
		t.Fatalf("ResolveNow requests = %d, want 1", resolver.refresh)
	}
}

func TestRuntimeSnapshotReportsDegradedWhenAnyBackendIsDegraded(t *testing.T) {
	resolver := &statusNodeResolver{statuses: []discovery.ResolverStatus{
		{Source: "static", State: discovery.BackendHealthy},
		{Source: "mdns", State: discovery.BackendDegraded, LastErrorCode: discovery.ErrorCodeQueryTimedOut},
	}}
	client := &LumenClient{pool: &Pool{}, resolver: resolver}
	if state := client.RuntimeSnapshot().DiscoveryState; state != discovery.BackendDegraded {
		t.Fatalf("discovery state = %q, want degraded", state)
	}
}

func TestAggregateDiscoveryStateIsOrderIndependent(t *testing.T) {
	for _, statuses := range [][]discovery.ResolverStatus{
		{
			{Source: "disabled", State: discovery.BackendDisabled},
			{Source: "healthy", State: discovery.BackendHealthy},
		},
		{
			{Source: "healthy", State: discovery.BackendHealthy},
			{Source: "disabled", State: discovery.BackendDisabled},
		},
	} {
		if state := aggregateDiscoveryState(statuses); state != discovery.BackendHealthy {
			t.Fatalf("aggregate state = %q, want healthy for %+v", state, statuses)
		}
	}
}
