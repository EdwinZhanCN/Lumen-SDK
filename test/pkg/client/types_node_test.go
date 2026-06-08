package client_test

import (
	"sync"
	"testing"

	"github.com/edwinzhancn/lumen-sdk/pkg/discovery"
	pb "github.com/edwinzhancn/lumen-sdk/proto"
)

func TestNodeInfoIsActive(t *testing.T) {
	tests := []struct {
		name     string
		status   discovery.NodeStatus
		expected bool
	}{
		{"Active node", discovery.NodeStatusActive, true},
		{"Unknown node", discovery.NodeStatusUnknown, false},
		{"Starting node", discovery.NodeStatusStarting, false},
		{"Error node", discovery.NodeStatusError, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &discovery.NodeInfo{
				Status: tt.status,
			}
			if got := node.IsActive(); got != tt.expected {
				t.Errorf("IsActive() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestNodeInfoSupportsTask(t *testing.T) {
	node := &discovery.NodeInfo{
		Tasks: []*pb.IOTask{
			{Name: "task1"},
			{Name: "task2"},
		},
		Capabilities: []*pb.Capability{
			{
				Tasks: []*pb.IOTask{
					{Name: "task3"},
				},
			},
		},
	}

	tests := []struct {
		task     string
		expected bool
	}{
		{"task1", true},
		{"task2", true},
		{"task3", true},
		{"task4", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.task, func(t *testing.T) {
			if got := node.SupportsTask(tt.task); got != tt.expected {
				t.Errorf("SupportsTask(%s) = %v, want %v", tt.task, got, tt.expected)
			}
		})
	}
}

func TestNodeInfoSupportsTaskCaching(t *testing.T) {
	node := &discovery.NodeInfo{
		Tasks: []*pb.IOTask{
			{Name: "task1"},
		},
	}

	// First call should build cache
	if !node.SupportsTask("task1") {
		t.Error("Expected task1 to be supported")
	}

	// Second call should use cache
	if !node.SupportsTask("task1") {
		t.Error("Expected task1 to be supported on second call (cached)")
	}

	// Non-existent task
	if node.SupportsTask("task2") {
		t.Error("Expected task2 not to be supported")
	}
}

func TestNodeInfoGetConnections(t *testing.T) {
	node := &discovery.NodeInfo{}

	if connections := node.GetConnections(); connections != 0 {
		t.Errorf("Expected 0 connections, got %d", connections)
	}
}

func TestNodeInfoIncrementConnections(t *testing.T) {
	node := &discovery.NodeInfo{}

	node.IncrementConnections()
	if connections := node.GetConnections(); connections != 1 {
		t.Errorf("Expected 1 connection after increment, got %d", connections)
	}

	node.IncrementConnections()
	if connections := node.GetConnections(); connections != 2 {
		t.Errorf("Expected 2 connections after second increment, got %d", connections)
	}
}

func TestNodeInfoDecrementConnections(t *testing.T) {
	node := &discovery.NodeInfo{}

	node.IncrementConnections()
	node.IncrementConnections()
	node.DecrementConnections()

	if connections := node.GetConnections(); connections != 1 {
		t.Errorf("Expected 1 connection after decrement, got %d", connections)
	}

	node.DecrementConnections()
	if connections := node.GetConnections(); connections != 0 {
		t.Errorf("Expected 0 connections after second decrement, got %d", connections)
	}
}

func TestNodeInfoConnectionsConcurrency(t *testing.T) {
	node := &discovery.NodeInfo{}
	var wg sync.WaitGroup

	// Concurrent increments
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			node.IncrementConnections()
		}()
	}

	wg.Wait()

	if connections := node.GetConnections(); connections != 100 {
		t.Errorf("Expected 100 connections after concurrent increments, got %d", connections)
	}

	// Concurrent decrements
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			node.DecrementConnections()
		}()
	}

	wg.Wait()

	if connections := node.GetConnections(); connections != 0 {
		t.Errorf("Expected 0 connections after concurrent decrements, got %d", connections)
	}
}

func TestNodeStatusConstants(t *testing.T) {
	statuses := []discovery.NodeStatus{
		discovery.NodeStatusUnknown,
		discovery.NodeStatusStarting,
		discovery.NodeStatusActive,
		discovery.NodeStatusError,
	}

	// Verify constants are different
	seen := make(map[discovery.NodeStatus]bool)
	for _, status := range statuses {
		if seen[status] {
			t.Errorf("Duplicate status value: %s", status)
		}
		seen[status] = true
	}

	// Verify they're all non-empty strings
	for _, status := range statuses {
		if string(status) == "" {
			t.Error("Status constant should not be empty")
		}
	}
}

func TestModelInfoStruct(t *testing.T) {
	model := discovery.ModelInfo{
		ID:      "model-123",
		Name:    "Test Model",
		Version: "1.0.0",
		Runtime: "cuda",
	}

	if model.ID != "model-123" {
		t.Errorf("Expected ID 'model-123', got %s", model.ID)
	}
	if model.Name != "Test Model" {
		t.Errorf("Expected Name 'Test Model', got %s", model.Name)
	}
	if model.Version != "1.0.0" {
		t.Errorf("Expected Version '1.0.0', got %s", model.Version)
	}
	if model.Runtime != "cuda" {
		t.Errorf("Expected Runtime 'cuda', got %s", model.Runtime)
	}
}
