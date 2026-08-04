package client

import (
	"testing"

	pb "github.com/edwinzhancn/lumen-sdk/proto"
)

func TestCompatibilityFromTXT(t *testing.T) {
	tests := []struct {
		name string
		txt  map[string]string
		want bool
	}{
		{"no hint (legacy)", map[string]string{"v": "0.1.1"}, true},
		{"empty hint", map[string]string{"proto": ""}, true},
		{"supported major", map[string]string{"proto": "1.0"}, true},
		{"supported plain major", map[string]string{"proto": "1"}, true},
		{"future major", map[string]string{"proto": "2.0"}, false},
		{"far future major", map[string]string{"proto": "3.0.0"}, false},
		{"unparsable", map[string]string{"proto": "banana"}, false},
		{"unparsable empty", map[string]string{"proto": "v"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compatibilityFromTXT(tt.txt)
			if got.compatible != tt.want {
				t.Fatalf("compatibilityFromTXT(%v) = %+v, want compatible=%v", tt.txt, got, tt.want)
			}
			if !got.compatible && got.incompatReason() == "" {
				t.Fatalf("incompatible node must carry a reason: %+v", got)
			}
		})
	}
}

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
