package client

import (
	"strings"
	"testing"

	"github.com/edwinzhancn/lumen-sdk/pkg/discovery"
	pb "github.com/edwinzhancn/lumen-sdk/proto"
)

func TestCompatibilityFromCapabilities(t *testing.T) {
	tests := []struct {
		name       string
		caps       []*pb.Capability
		want       discovery.CompatibilityState
		reasonPart string
	}{
		{"no caps", nil, discovery.CompatibilityIncompatible, "no capabilities"},
		{"missing version", []*pb.Capability{{ProtocolVersion: ""}}, discovery.CompatibilityIncompatible, "omitted"},
		{"supported version", []*pb.Capability{{ProtocolVersion: "1.0"}}, discovery.CompatibilityCompatible, ""},
		{"mixed supported", []*pb.Capability{{ProtocolVersion: "1.0"}, {ProtocolVersion: "1.0.3"}}, discovery.CompatibilityCompatible, ""},
		{"future major", []*pb.Capability{{ProtocolVersion: "2.0"}}, discovery.CompatibilityIncompatible, "major 2"},
		{"mixed major", []*pb.Capability{{ProtocolVersion: "1.0"}, {ProtocolVersion: "2.0"}}, discovery.CompatibilityIncompatible, "major 2"},
		{"unparsable", []*pb.Capability{{ProtocolVersion: "not-a-version"}}, discovery.CompatibilityIncompatible, "unparsable"},
		{"nil capability", []*pb.Capability{nil}, discovery.CompatibilityIncompatible, "no capabilities"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, reason := compatibilityFromCapabilities(tt.caps)
			if state != tt.want {
				t.Fatalf("state = %q, want %q (reason %q)", state, tt.want, reason)
			}
			if tt.reasonPart != "" && !strings.Contains(reason, tt.reasonPart) {
				t.Fatalf("reason = %q, want substring %q", reason, tt.reasonPart)
			}
			if tt.reasonPart == "" && reason != "" {
				t.Fatalf("compatible result has reason %q", reason)
			}
		})
	}
}
