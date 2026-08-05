package client

import (
	"testing"

	pb "github.com/edwinzhancn/lumen-sdk/proto"
)

func TestCompatibilityFromCapabilities(t *testing.T) {
	tests := []struct {
		name string
		caps []*pb.Capability
		want bool
	}{
		{"no caps", nil, true},
		{"legacy empty version", []*pb.Capability{{ProtocolVersion: ""}}, true},
		{"supported version", []*pb.Capability{{ProtocolVersion: "1.0"}}, true},
		{"mixed supported", []*pb.Capability{{ProtocolVersion: "1.0"}, {ProtocolVersion: "1.0.3"}}, true},
		{"future major", []*pb.Capability{{ProtocolVersion: "2.0"}}, false},
		{"mixed major", []*pb.Capability{{ProtocolVersion: "1.0"}, {ProtocolVersion: "2.0"}}, false},
		{"unparsable", []*pb.Capability{{ProtocolVersion: "not-a-version"}}, false},
		{"nil capability", []*pb.Capability{nil}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compatibilityFromCapabilities(tt.caps)
			if got.compatible != tt.want {
				t.Fatalf("compatibilityFromCapabilities(%v) = %+v, want compatible=%v", tt.caps, got, tt.want)
			}
		})
	}
}
