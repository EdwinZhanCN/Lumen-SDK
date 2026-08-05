package discovery

import (
	pb "github.com/edwinzhancn/lumen-sdk/proto"
	"google.golang.org/protobuf/proto"
)

func CloneNodeSlice(nodes []*NodeInfo) []*NodeInfo {
	if len(nodes) == 0 {
		return []*NodeInfo{}
	}

	cloned := make([]*NodeInfo, 0, len(nodes))
	for _, node := range nodes {
		if node != nil {
			cloned = append(cloned, CloneNode(node))
		}
	}
	return cloned
}

func CloneNode(node *NodeInfo) *NodeInfo {
	if node == nil {
		return nil
	}
	out := &NodeInfo{
		ID:                 node.ID,
		Address:            node.Address,
		Availability:       node.Availability,
		Compatibility:      node.Compatibility,
		Version:            node.Version,
		Runtime:            node.Runtime,
		UpdatedAt:          node.UpdatedAt,
		IncompatibleReason: node.IncompatibleReason,
	}

	if node.Metadata != nil {
		out.Metadata = make(map[string]interface{}, len(node.Metadata))
		for k, v := range node.Metadata {
			out.Metadata[k] = v
		}
	}
	if len(node.Models) > 0 {
		out.Models = make([]*ModelInfo, 0, len(node.Models))
		for _, model := range node.Models {
			if model != nil {
				copied := *model
				out.Models = append(out.Models, &copied)
			}
		}
	}
	out.Capabilities = CloneCapabilities(node.Capabilities)
	return out
}

func CloneCapabilities(caps []*pb.Capability) []*pb.Capability {
	if len(caps) == 0 {
		return nil
	}
	out := make([]*pb.Capability, 0, len(caps))
	for _, capability := range caps {
		if capability == nil {
			continue
		}
		cloned, ok := proto.Clone(capability).(*pb.Capability)
		if ok {
			out = append(out, cloned)
		}
	}
	return out
}
