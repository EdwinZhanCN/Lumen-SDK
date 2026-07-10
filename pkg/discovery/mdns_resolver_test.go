package discovery

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/hashicorp/mdns"
)

func TestParseNodeIdentity(t *testing.T) {
	tests := []struct {
		name       string
		instance   string
		defaultDep string
		want       NodeIdentity
	}{
		{"deployment node", "lab-node-1", "lab", NodeIdentity{DeploymentID: "lab", NodeID: "node-1"}},
		{"legacy node", "node-1", "local", NodeIdentity{DeploymentID: "local", NodeID: "node-1"}},
		{"empty default", "node-1", "", NodeIdentity{DeploymentID: "local", NodeID: "node-1"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseNodeIdentity(tt.instance, tt.defaultDep)
			if got != tt.want {
				t.Fatalf("ParseNodeIdentity() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestMDNSResolvedNodeKeepsCandidatesAndTXT(t *testing.T) {
	resolver := &MDNSResolver{
		serviceType:  "_lumen._tcp",
		domain:       "local",
		deploymentID: "lab",
	}
	entry := &mdns.ServiceEntry{
		Name:       "lab-node-1._lumen._tcp.local.",
		Host:       "host.local.",
		Port:       5866,
		AddrV4:     net.ParseIP("192.168.1.20"),
		AddrV6:     net.ParseIP("fd00::1"),
		InfoFields: []string{"v=1.2.3", "runtime=onnxrt", "cap_hash=abc", "tasks=ocr, embed"},
	}

	resolved := resolver.resolvedNodeFromMDNS(entry)
	if resolved.Key() != "lab-node-1" {
		t.Fatalf("key = %q, want lab-node-1", resolved.Key())
	}
	endpoints := resolved.CandidateEndpoints()
	wantEndpoints := []string{"192.168.1.20:5866", "[fd00::1]:5866"}
	if len(endpoints) != len(wantEndpoints) {
		t.Fatalf("endpoints = %#v, want %#v", endpoints, wantEndpoints)
	}
	for i := range wantEndpoints {
		if endpoints[i] != wantEndpoints[i] {
			t.Fatalf("endpoints = %#v, want %#v", endpoints, wantEndpoints)
		}
	}
	if resolved.CapHash() != "abc" || resolved.Version() != "1.2.3" || resolved.Runtime() != "onnxrt" {
		t.Fatalf("TXT not parsed correctly: %#v", resolved.Txt)
	}
	if tasks := resolved.HintTasks(); len(tasks) != 2 || tasks[0] != "ocr" || tasks[1] != "embed" {
		t.Fatalf("HintTasks() = %#v", tasks)
	}
}

func TestMDNSResolvedNodeMissingAddressIsNotEndpoint(t *testing.T) {
	resolver := &MDNSResolver{
		serviceType:  "_lumen._tcp",
		domain:       "local",
		deploymentID: "local",
	}
	entry := &mdns.ServiceEntry{
		Name: "node-1._lumen._tcp.local.",
		Host: "host.local.",
		Port: 5866,
	}

	resolved := resolver.resolvedNodeFromMDNS(entry)
	if resolved.Key() != "local-node-1" {
		t.Fatalf("key = %q, want local-node-1", resolved.Key())
	}
	if endpoint := resolved.Endpoint(); endpoint != "" {
		t.Fatalf("Endpoint() = %q, want empty", endpoint)
	}
}

func TestEventFromResolvedTTLExpiryShape(t *testing.T) {
	resolved := ResolvedNode{
		Identity:  NewNodeIdentity("local", "node-1"),
		Addresses: []string{"127.0.0.1"},
		Port:      5866,
		Txt:       map[string]string{"tasks": "ocr"},
	}

	ev := eventFromResolved(NodeExpired, resolved)
	if ev.Type != NodeExpired {
		t.Fatalf("event type = %v, want NodeExpired", ev.Type)
	}
	if ev.ExplicitRemove {
		t.Fatal("mDNS-style expiry should not be explicit remove")
	}
	if len(ev.Addresses) != 1 || ev.Addresses[0] != "127.0.0.1:5866" {
		t.Fatalf("Addresses = %v, want [127.0.0.1:5866]", ev.Addresses)
	}
}

func TestExtractInstanceName(t *testing.T) {
	tests := []struct {
		name        string
		fullName    string
		serviceType string
		domain      string
		want        string
	}{
		{"full DNS-SD name", "lab-node-1._lumen._tcp.local.", "_lumen._tcp", "local", "lab-node-1"},
		{"no trailing dot", "lab-node-1._lumen._tcp.local", "_lumen._tcp", "local", "lab-node-1"},
		{"different service", "node-1._http._tcp.local.", "_http._tcp", "local", "node-1"},
		{"no suffix match", "node-1.other.local.", "_lumen._tcp", "local", "node-1"},
		{"just instance", "node-1", "_lumen._tcp", "local", "node-1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractInstanceName(tt.fullName, tt.serviceType, tt.domain)
			if got != tt.want {
				t.Fatalf("extractInstanceName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMDNSResolvedNodeIPv6Only(t *testing.T) {
	resolver := &MDNSResolver{
		serviceType:  "_lumen._tcp",
		domain:       "local",
		deploymentID: "local",
	}
	entry := &mdns.ServiceEntry{
		Name:         "node-1._lumen._tcp.local.",
		Host:         "host.local.",
		Port:         5866,
		AddrV6IPAddr: &net.IPAddr{IP: net.ParseIP("fe80::1"), Zone: "en0"},
		InfoFields:   []string{"v=1.0"},
	}

	resolved := resolver.resolvedNodeFromMDNS(entry)
	endpoints := resolved.CandidateEndpoints()
	if len(endpoints) != 1 {
		t.Fatalf("expected 1 endpoint, got %d: %v", len(endpoints), endpoints)
	}
	if resolved.Version() != "1.0" {
		t.Fatalf("version = %q, want 1.0", resolved.Version())
	}
}

func TestPollLoopContextCancellation(t *testing.T) {
	resolver := &MDNSResolver{
		serviceType:  "_nonexistent._tcp",
		domain:       "local",
		deploymentID: "test",
		pollInterval: 100 * time.Millisecond,
		queryTimeout: 100 * time.Millisecond,
	}

	ctx, cancel := context.WithCancel(context.Background())
	ch, err := resolver.Watch(ctx)
	if err != nil {
		t.Fatalf("Watch() error: %v", err)
	}

	cancel()

	// Channel should close promptly after context cancellation.
	select {
	case _, ok := <-ch:
		if ok {
			// Drain any buffered events
			for range ch {
			}
		}
	case <-time.After(5 * time.Second):
		t.Fatal("channel not closed after context cancellation")
	}
}
