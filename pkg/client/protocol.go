package client

import (
	"fmt"
	"strings"

	"github.com/edwinzhancn/lumen-sdk/pkg/discovery"
	pb "github.com/edwinzhancn/lumen-sdk/proto"
)

// SupportedDataPlaneMajor is the only protocol major routed by this SDK.
const SupportedDataPlaneMajor = 1

// compatibilityFromCapabilities accepts only explicit, parseable protocol
// versions. There is no legacy empty-version path: capability exchange is the
// data-plane compatibility authority.
func compatibilityFromCapabilities(caps []*pb.Capability) (discovery.CompatibilityState, string) {
	seen := false
	for _, capability := range caps {
		if capability == nil {
			continue
		}
		seen = true
		version := strings.TrimSpace(capability.GetProtocolVersion())
		if version == "" {
			return discovery.CompatibilityIncompatible, "capability omitted data-plane protocol version"
		}
		major, ok := discovery.ParseProtocolMajor(version)
		if !ok {
			return discovery.CompatibilityIncompatible, fmt.Sprintf("unparsable data-plane protocol version %q", version)
		}
		if major != SupportedDataPlaneMajor {
			return discovery.CompatibilityIncompatible, fmt.Sprintf(
				"unsupported data-plane protocol major %d (SDK speaks %d)",
				major, SupportedDataPlaneMajor,
			)
		}
	}
	if !seen {
		return discovery.CompatibilityIncompatible, "node reported no capabilities"
	}
	return discovery.CompatibilityCompatible, ""
}
