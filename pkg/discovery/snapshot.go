package discovery

import (
	"sync/atomic"

	pb "github.com/edwinzhancn/lumen-sdk/proto"
	"google.golang.org/protobuf/proto"
)

func CloneNodeSlice(nodes []*NodeInfo) []*NodeInfo {
	if len(nodes) == 0 {
		return []*NodeInfo{}
	}

	cloned := make([]*NodeInfo, 0, len(nodes))
	for _, node := range nodes {
		if node == nil {
			continue
		}
		cloned = append(cloned, CloneNode(node))
	}
	return cloned
}

func CloneNode(node *NodeInfo) *NodeInfo {
	node.mu.RLock()
	defer node.mu.RUnlock()

	out := &NodeInfo{
		ID:       node.ID,
		Name:     node.Name,
		Address:  node.Address,
		Status:   node.Status,
		Version:  node.Version,
		Runtime:  node.Runtime,
		LastSeen: node.LastSeen,
		Weight:   node.Weight,
	}

	if node.Metadata != nil {
		out.Metadata = make(map[string]interface{}, len(node.Metadata))
		for k, v := range node.Metadata {
			out.Metadata[k] = v
		}
	}

	if len(node.Models) > 0 {
		out.Models = make([]*ModelInfo, 0, len(node.Models))
		for _, m := range node.Models {
			if m == nil {
				continue
			}
			copied := *m
			out.Models = append(out.Models, &copied)
		}
	}

	out.Tasks = CloneIOTasks(node.Tasks)
	out.Capabilities = CloneCapabilities(node.Capabilities)

	if node.Load != nil {
		loadCopy := *node.Load
		out.Load = &loadCopy
	}

	if node.Stats != nil {
		statsCopy := *node.Stats
		out.Stats = &statsCopy
	}

	out.connections = atomic.LoadInt64(&node.connections)
	return out
}

func CloneCapabilities(caps []*pb.Capability) []*pb.Capability {
	if len(caps) == 0 {
		return nil
	}

	out := make([]*pb.Capability, 0, len(caps))
	for _, cap := range caps {
		if cap == nil {
			continue
		}
		cloned, ok := proto.Clone(cap).(*pb.Capability)
		if !ok {
			continue
		}
		out = append(out, cloned)
	}
	return out
}

func CloneIOTasks(tasks []*pb.IOTask) []*pb.IOTask {
	if len(tasks) == 0 {
		return nil
	}

	out := make([]*pb.IOTask, 0, len(tasks))
	for _, task := range tasks {
		if task == nil {
			continue
		}
		cloned, ok := proto.Clone(task).(*pb.IOTask)
		if !ok {
			continue
		}
		out = append(out, cloned)
	}
	return out
}
