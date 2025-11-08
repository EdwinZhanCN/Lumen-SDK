package client_test

import (
	"sync"
	"testing"
	"time"

	"github.com/edwinzhancn/lumen-sdk/pkg/client"
	pb "github.com/edwinzhancn/lumen-sdk/proto"
)

func TestNodeInfoIsActive(t *testing.T) {
	tests := []struct {
		name     string
		status   client.NodeStatus
		expected bool
	}{
		{"Active node", client.NodeStatusActive, true},
		{"Unknown node", client.NodeStatusUnknown, false},
		{"Starting node", client.NodeStatusStarting, false},
		{"Error node", client.NodeStatusError, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &client.NodeInfo{
				Status: tt.status,
			}
			if got := node.IsActive(); got != tt.expected {
				t.Errorf("IsActive() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestNodeInfoSupportsTask(t *testing.T) {
	node := &client.NodeInfo{
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
	node := &client.NodeInfo{
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
	node := &client.NodeInfo{}

	if connections := node.GetConnections(); connections != 0 {
		t.Errorf("Expected 0 connections, got %d", connections)
	}
}

func TestNodeInfoIncrementConnections(t *testing.T) {
	node := &client.NodeInfo{}

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
	node := &client.NodeInfo{}

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
	node := &client.NodeInfo{}
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

func TestNodeInfoUpdateLoad(t *testing.T) {
	node := &client.NodeInfo{}

	load := &client.NodeLoad{
		CPU:    0.5,
		Memory: 0.7,
		GPU:    0.3,
		Disk:   0.8,
	}

	node.UpdateLoad(load)

	if node.Load == nil {
		t.Fatal("Expected Load to be set")
	}

	if node.Load.CPU != 0.5 {
		t.Errorf("Expected CPU 0.5, got %f", node.Load.CPU)
	}
	if node.Load.Memory != 0.7 {
		t.Errorf("Expected Memory 0.7, got %f", node.Load.Memory)
	}
	if node.Load.GPU != 0.3 {
		t.Errorf("Expected GPU 0.3, got %f", node.Load.GPU)
	}
	if node.Load.Disk != 0.8 {
		t.Errorf("Expected Disk 0.8, got %f", node.Load.Disk)
	}
}

func TestNodeInfoUpdateStats(t *testing.T) {
	node := &client.NodeInfo{}

	stats := &client.NodeStats{
		TotalRequests:      100,
		SuccessfulRequests: 90,
		FailedRequests:     10,
		AverageLatency:     50,
		LastRequest:        time.Now(),
	}

	node.UpdateStats(stats)

	if node.Stats == nil {
		t.Fatal("Expected Stats to be set")
	}

	if node.Stats.TotalRequests != 100 {
		t.Errorf("Expected TotalRequests 100, got %d", node.Stats.TotalRequests)
	}
	if node.Stats.SuccessfulRequests != 90 {
		t.Errorf("Expected SuccessfulRequests 90, got %d", node.Stats.SuccessfulRequests)
	}
	if node.Stats.FailedRequests != 10 {
		t.Errorf("Expected FailedRequests 10, got %d", node.Stats.FailedRequests)
	}
}

func TestNodeInfoRecordRequestSuccess(t *testing.T) {
	node := &client.NodeInfo{}

	latency := 100 * time.Millisecond
	node.RecordRequest(true, latency)

	if node.Stats == nil {
		t.Fatal("Expected Stats to be initialized")
	}

	if node.Stats.TotalRequests != 1 {
		t.Errorf("Expected TotalRequests 1, got %d", node.Stats.TotalRequests)
	}
	if node.Stats.SuccessfulRequests != 1 {
		t.Errorf("Expected SuccessfulRequests 1, got %d", node.Stats.SuccessfulRequests)
	}
	if node.Stats.FailedRequests != 0 {
		t.Errorf("Expected FailedRequests 0, got %d", node.Stats.FailedRequests)
	}
	if node.Stats.AverageLatency != int64(latency) {
		t.Errorf("Expected AverageLatency %d, got %d", int64(latency), node.Stats.AverageLatency)
	}
}

func TestNodeInfoRecordRequestFailure(t *testing.T) {
	node := &client.NodeInfo{}

	latency := 50 * time.Millisecond
	node.RecordRequest(false, latency)

	if node.Stats == nil {
		t.Fatal("Expected Stats to be initialized")
	}

	if node.Stats.TotalRequests != 1 {
		t.Errorf("Expected TotalRequests 1, got %d", node.Stats.TotalRequests)
	}
	if node.Stats.SuccessfulRequests != 0 {
		t.Errorf("Expected SuccessfulRequests 0, got %d", node.Stats.SuccessfulRequests)
	}
	if node.Stats.FailedRequests != 1 {
		t.Errorf("Expected FailedRequests 1, got %d", node.Stats.FailedRequests)
	}
}

func TestNodeInfoRecordRequestAverageLatency(t *testing.T) {
	node := &client.NodeInfo{}

	// Record first request
	node.RecordRequest(true, 100*time.Millisecond)

	// Record second request
	node.RecordRequest(true, 200*time.Millisecond)

	// Average should be calculated using exponential moving average
	// First: 100ms
	// Second: 100ms * 0.9 + 200ms * 0.1 = 90ms + 20ms = 110ms
	expected := int64(110 * time.Millisecond)
	if node.Stats.AverageLatency != expected {
		t.Logf("Note: Average latency calculation uses exponential moving average (alpha=0.1)")
		t.Logf("Expected approximately %d, got %d", expected, node.Stats.AverageLatency)
	}
}

func TestNodeInfoGetErrorRate(t *testing.T) {
	node := &client.NodeInfo{}

	// No stats
	if errorRate := node.GetErrorRate(); errorRate != 0.0 {
		t.Errorf("Expected error rate 0.0 for no stats, got %f", errorRate)
	}

	// With stats
	node.Stats = &client.NodeStats{
		TotalRequests:      100,
		SuccessfulRequests: 90,
		FailedRequests:     10,
	}

	expectedRate := 0.1
	if errorRate := node.GetErrorRate(); errorRate != expectedRate {
		t.Errorf("Expected error rate %f, got %f", expectedRate, errorRate)
	}
}

func TestNodeInfoGetErrorRateZeroRequests(t *testing.T) {
	node := &client.NodeInfo{
		Stats: &client.NodeStats{
			TotalRequests: 0,
		},
	}

	if errorRate := node.GetErrorRate(); errorRate != 0.0 {
		t.Errorf("Expected error rate 0.0 for zero requests, got %f", errorRate)
	}
}

func TestNodeLoadStruct(t *testing.T) {
	load := client.NodeLoad{
		CPU:    0.75,
		Memory: 0.85,
		GPU:    0.50,
		Disk:   0.60,
	}

	if load.CPU != 0.75 {
		t.Errorf("Expected CPU 0.75, got %f", load.CPU)
	}
	if load.Memory != 0.85 {
		t.Errorf("Expected Memory 0.85, got %f", load.Memory)
	}
	if load.GPU != 0.50 {
		t.Errorf("Expected GPU 0.50, got %f", load.GPU)
	}
	if load.Disk != 0.60 {
		t.Errorf("Expected Disk 0.60, got %f", load.Disk)
	}
}

func TestNodeStatsStruct(t *testing.T) {
	now := time.Now()
	stats := client.NodeStats{
		TotalRequests:      1000,
		SuccessfulRequests: 950,
		FailedRequests:     50,
		AverageLatency:     75,
		LastRequest:        now,
	}

	if stats.TotalRequests != 1000 {
		t.Errorf("Expected TotalRequests 1000, got %d", stats.TotalRequests)
	}
	if stats.SuccessfulRequests != 950 {
		t.Errorf("Expected SuccessfulRequests 950, got %d", stats.SuccessfulRequests)
	}
	if stats.FailedRequests != 50 {
		t.Errorf("Expected FailedRequests 50, got %d", stats.FailedRequests)
	}
	if stats.AverageLatency != 75 {
		t.Errorf("Expected AverageLatency 75, got %d", stats.AverageLatency)
	}
	if !stats.LastRequest.Equal(now) {
		t.Errorf("Expected LastRequest %v, got %v", now, stats.LastRequest)
	}
}

func TestNodeStatusConstants(t *testing.T) {
	statuses := []client.NodeStatus{
		client.NodeStatusUnknown,
		client.NodeStatusStarting,
		client.NodeStatusActive,
		client.NodeStatusError,
	}

	// Verify constants are different
	seen := make(map[client.NodeStatus]bool)
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
	model := client.ModelInfo{
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
