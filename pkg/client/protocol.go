package client

import (
	"fmt"

	"github.com/edwinzhancn/lumen-sdk/pkg/discovery"
	pb "github.com/edwinzhancn/lumen-sdk/proto"
)

// SupportedDataPlaneMajor is the data-plane protocol major this SDK speaks,
// mirroring the `home_native.vN` gRPC package in proto/ml_service.proto.
// Bumping it requires a protocol-major release: hubs announcing a different
// major are never added to the task pool.
const SupportedDataPlaneMajor = 1

// protocolCompatibility describes whether a discovered node can be scheduled.
type protocolCompatibility struct {
	compatible bool
	reason     string
}

func (c protocolCompatibility) incompatReason() string {
	if c.compatible {
		return ""
	}
	if c.reason == "" {
		return "incompatible node"
	}
	return c.reason
}

// compatibilityFromCapabilities evaluates the per-capability protocol_version
// reported over the data plane. A node is compatible only when every reported
// version parses to the supported major; empty versions (legacy hubs) are
// tolerated.
func compatibilityFromCapabilities(caps []*pb.Capability) protocolCompatibility {
	for _, capability := range caps {
		if capability == nil {
			continue
		}
		version := capability.GetProtocolVersion()
		if version == "" {
			continue
		}
		major, ok := discovery.ParseProtocolMajor(version)
		if !ok {
			return protocolCompatibility{
				compatible: false,
				reason:     fmt.Sprintf("unparsable data-plane protocol version %q", version),
			}
		}
		if major != SupportedDataPlaneMajor {
			return protocolCompatibility{
				compatible: false,
				reason: fmt.Sprintf(
					"unsupported data-plane protocol major %d (SDK speaks %d)",
					major, SupportedDataPlaneMajor,
				),
			}
		}
	}
	return protocolCompatibility{compatible: true}
}
