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

// compatibilityFromTXT evaluates the protocol hint announced in discovery TXT
// records. Hubs that publish the `proto` key are decided immediately: an
// unparsable or unsupported major keeps the node visible but out of the pool.
// Hubs without the key (legacy releases) stay optimistic and are decided by
// the capability fetch instead.
func compatibilityFromTXT(txt map[string]string) protocolCompatibility {
	version := txt[discovery.ProtocolVersionTXTKey]
	if version == "" {
		return protocolCompatibility{compatible: true}
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
	return protocolCompatibility{compatible: true}
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
