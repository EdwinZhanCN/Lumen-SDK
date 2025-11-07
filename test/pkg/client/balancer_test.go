package client_test

import (
	"context"
	"testing"
	"time"

	"github.com/edwinzhancn/lumen-sdk/pkg/client"
	"github.com/edwinzhancn/lumen-sdk/pkg/config"
	pb "github.com/edwinzhancn/lumen-sdk/proto"
	"go.uber.org/zap"
)

func createTestNodes(count int) []*client.NodeInfo {
	nodes := make([]*client.NodeInfo, count)
	for i := 0; i < count; i++ {
		nodes[i] = &client.NodeInfo{
			ID:      string(rune('A' + i)),
			Name:    string(rune('A' + i)),
			Status:  client.NodeStatusActive,
			Weight:  int64(i + 1), // Weight increases: 1, 2, 3, ...
			Tasks: []*pb.IOTask{
				{Name: "test_task"},
			},
		}
	}
	return nodes
}

func TestRoundRobinStrategy(t *testing.T) {
	strategy := client.NewRoundRobinStrategy()
	nodes := createTestNodes(3)
	ctx := context.Background()

	// Test sequential selection
	for i := 0; i < 6; i++ {
		node, err := strategy.Select(ctx, nodes, "test_task")
		if err != nil {
			t.Fatalf("Select() error = %v", err)
		}

		expectedIndex := i % 3
		if node.ID != nodes[expectedIndex].ID {
			t.Errorf("Iteration %d: expected node %s, got %s", i, nodes[expectedIndex].ID, node.ID)
		}
	}
}

func TestRoundRobinStrategyName(t *testing.T) {
	strategy := client.NewRoundRobinStrategy()
	if name := strategy.Name(); name != "round_robin" {
		t.Errorf("Expected name 'round_robin', got %s", name)
	}
}

func TestRoundRobinStrategyEmptyNodes(t *testing.T) {
	strategy := client.NewRoundRobinStrategy()
	ctx := context.Background()

	_, err := strategy.Select(ctx, []*client.NodeInfo{}, "test_task")
	if err == nil {
		t.Error("Expected error for empty nodes, got nil")
	}
}

func TestRandomStrategy(t *testing.T) {
	strategy := client.NewRandomStrategy()
	nodes := createTestNodes(3)
	ctx := context.Background()

	// Test that selection works
	node, err := strategy.Select(ctx, nodes, "test_task")
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}

	// Verify node is one of the available nodes
	found := false
	for _, n := range nodes {
		if n.ID == node.ID {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Selected node %s not in available nodes", node.ID)
	}
}

func TestRandomStrategyName(t *testing.T) {
	strategy := client.NewRandomStrategy()
	if name := strategy.Name(); name != "random" {
		t.Errorf("Expected name 'random', got %s", name)
	}
}

func TestRandomStrategyDistribution(t *testing.T) {
	strategy := client.NewRandomStrategy()
	nodes := createTestNodes(3)
	ctx := context.Background()

	// Select many times and verify we get different nodes
	selections := make(map[string]int)
	for i := 0; i < 300; i++ {
		node, err := strategy.Select(ctx, nodes, "test_task")
		if err != nil {
			t.Fatalf("Select() error = %v", err)
		}
		selections[node.ID]++
	}

	// Each node should be selected at least once in 300 selections
	for _, node := range nodes {
		if selections[node.ID] == 0 {
			t.Errorf("Node %s was never selected in 300 iterations", node.ID)
		}
	}
}

func TestWeightedStrategy(t *testing.T) {
	strategy := client.NewWeightedStrategy()
	nodes := createTestNodes(3)
	ctx := context.Background()

	// Test that selection works
	node, err := strategy.Select(ctx, nodes, "test_task")
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}

	// Verify node is one of the available nodes
	found := false
	for _, n := range nodes {
		if n.ID == node.ID {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Selected node %s not in available nodes", node.ID)
	}
}

func TestWeightedStrategyName(t *testing.T) {
	strategy := client.NewWeightedStrategy()
	if name := strategy.Name(); name != "weighted" {
		t.Errorf("Expected name 'weighted', got %s", name)
	}
}

func TestWeightedStrategyNoWeights(t *testing.T) {
	strategy := client.NewWeightedStrategy()
	// Create nodes with no weights (weight = 0)
	nodes := []*client.NodeInfo{
		{ID: "A", Status: client.NodeStatusActive, Weight: 0, Tasks: []*pb.IOTask{{Name: "test_task"}}},
		{ID: "B", Status: client.NodeStatusActive, Weight: 0, Tasks: []*pb.IOTask{{Name: "test_task"}}},
	}
	ctx := context.Background()

	// Should fall back to random selection
	node, err := strategy.Select(ctx, nodes, "test_task")
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}

	if node == nil {
		t.Error("Expected a node to be selected despite zero weights")
	}
}

func TestLeastConnectionsStrategy(t *testing.T) {
	strategy := client.NewLeastConnectionsStrategy()
	nodes := createTestNodes(3)

	// Set different connection counts
	nodes[0].IncrementConnections()
	nodes[0].IncrementConnections()
	nodes[1].IncrementConnections()
	// nodes[2] has 0 connections

	ctx := context.Background()
	node, err := strategy.Select(ctx, nodes, "test_task")
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}

	// Should select node with least connections (nodes[2])
	if node.ID != nodes[2].ID {
		t.Errorf("Expected node %s with 0 connections, got %s", nodes[2].ID, node.ID)
	}
}

func TestLeastConnectionsStrategyName(t *testing.T) {
	strategy := client.NewLeastConnectionsStrategy()
	if name := strategy.Name(); name != "least_connections" {
		t.Errorf("Expected name 'least_connections', got %s", name)
	}
}

func TestSimpleLoadBalancerSelectNode(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	strategy := client.NewRoundRobinStrategy()
	cfg := &config.LoadBalancerConfig{
		CacheEnabled: false,
		HealthCheck:  false,
	}

	lb := client.NewSimpleLoadBalancer(strategy, cfg, logger)
	nodes := createTestNodes(3)
	lb.UpdateNodes(nodes)

	ctx := context.Background()
	node, err := lb.SelectNode(ctx, "test_task")
	if err != nil {
		t.Fatalf("SelectNode() error = %v", err)
	}

	if node == nil {
		t.Error("Expected a node to be selected")
	}
}

func TestSimpleLoadBalancerSelectNodeNoNodes(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	strategy := client.NewRoundRobinStrategy()
	cfg := &config.LoadBalancerConfig{
		CacheEnabled: false,
		HealthCheck:  false,
	}

	lb := client.NewSimpleLoadBalancer(strategy, cfg, logger)

	ctx := context.Background()
	_, err := lb.SelectNode(ctx, "test_task")
	if err == nil {
		t.Error("Expected error when no nodes available, got nil")
	}
}

func TestSimpleLoadBalancerUpdateNodes(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	strategy := client.NewRoundRobinStrategy()
	cfg := &config.LoadBalancerConfig{
		CacheEnabled: false,
		HealthCheck:  false,
	}

	lb := client.NewSimpleLoadBalancer(strategy, cfg, logger)
	nodes := createTestNodes(5)
	lb.UpdateNodes(nodes)

	stats := lb.GetStats()
	if stats.TotalNodes != 5 {
		t.Errorf("Expected TotalNodes 5, got %d", stats.TotalNodes)
	}
	if stats.ActiveNodes != 5 {
		t.Errorf("Expected ActiveNodes 5, got %d", stats.ActiveNodes)
	}
}

func TestSimpleLoadBalancerGetStats(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	strategy := client.NewRoundRobinStrategy()
	cfg := &config.LoadBalancerConfig{
		CacheEnabled: false,
		HealthCheck:  false,
	}

	lb := client.NewSimpleLoadBalancer(strategy, cfg, logger)
	nodes := createTestNodes(3)

	// Set one node as inactive
	nodes[1].Status = client.NodeStatusError

	lb.UpdateNodes(nodes)

	stats := lb.GetStats()
	if stats.TotalNodes != 3 {
		t.Errorf("Expected TotalNodes 3, got %d", stats.TotalNodes)
	}
	if stats.ActiveNodes != 2 {
		t.Errorf("Expected ActiveNodes 2 (one is error), got %d", stats.ActiveNodes)
	}
	if stats.Strategy != "round_robin" {
		t.Errorf("Expected Strategy 'round_robin', got %s", stats.Strategy)
	}
}

func TestSimpleLoadBalancerClose(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	strategy := client.NewRoundRobinStrategy()
	cfg := &config.LoadBalancerConfig{
		CacheEnabled: false,
		HealthCheck:  false,
	}

	lb := client.NewSimpleLoadBalancer(strategy, cfg, logger)

	err := lb.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Should not be able to select after close
	ctx := context.Background()
	_, err = lb.SelectNode(ctx, "test_task")
	if err == nil {
		t.Error("Expected error after Close(), got nil")
	}
}

func TestSimpleLoadBalancerWithCache(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	strategy := client.NewRoundRobinStrategy()
	cfg := &config.LoadBalancerConfig{
		CacheEnabled: true,
		CacheTTL:     5 * time.Second,
		HealthCheck:  false,
	}

	lb := client.NewSimpleLoadBalancer(strategy, cfg, logger)
	nodes := createTestNodes(3)
	lb.UpdateNodes(nodes)

	ctx := context.Background()

	// First selection - should cache
	node1, err := lb.SelectNode(ctx, "test_task")
	if err != nil {
		t.Fatalf("SelectNode() error = %v", err)
	}

	// Second selection of same task - should return cached node
	node2, err := lb.SelectNode(ctx, "test_task")
	if err != nil {
		t.Fatalf("SelectNode() error = %v", err)
	}

	if node1.ID != node2.ID {
		t.Errorf("Expected same node from cache, got %s and %s", node1.ID, node2.ID)
	}
}

func TestCreateLoadBalancerRoundRobin(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := &config.LoadBalancerConfig{
		Strategy: "round_robin",
	}

	lb := client.CreateLoadBalancer(client.RoundRobin, cfg, logger)
	if lb == nil {
		t.Fatal("Expected load balancer to be created")
	}

	stats := lb.GetStats()
	if stats.Strategy != "round_robin" {
		t.Errorf("Expected strategy 'round_robin', got %s", stats.Strategy)
	}
}

func TestCreateLoadBalancerRandom(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := &config.LoadBalancerConfig{
		Strategy: "random",
	}

	lb := client.CreateLoadBalancer(client.Random, cfg, logger)
	if lb == nil {
		t.Fatal("Expected load balancer to be created")
	}

	stats := lb.GetStats()
	if stats.Strategy != "random" {
		t.Errorf("Expected strategy 'random', got %s", stats.Strategy)
	}
}

func TestCreateLoadBalancerWeighted(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := &config.LoadBalancerConfig{
		Strategy: "weighted",
	}

	lb := client.CreateLoadBalancer(client.Weighted, cfg, logger)
	if lb == nil {
		t.Fatal("Expected load balancer to be created")
	}

	stats := lb.GetStats()
	if stats.Strategy != "weighted" {
		t.Errorf("Expected strategy 'weighted', got %s", stats.Strategy)
	}
}

func TestCreateLoadBalancerLeastConnections(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := &config.LoadBalancerConfig{
		Strategy: "least_connections",
	}

	lb := client.CreateLoadBalancer(client.LeastConnections, cfg, logger)
	if lb == nil {
		t.Fatal("Expected load balancer to be created")
	}

	stats := lb.GetStats()
	if stats.Strategy != "least_connections" {
		t.Errorf("Expected strategy 'least_connections', got %s", stats.Strategy)
	}
}

func TestLoadBalancerTypeConstants(t *testing.T) {
	types := []client.LoadBalancerType{
		client.RoundRobin,
		client.Random,
		client.Weighted,
		client.LeastConnections,
		client.TaskAwareRoundRobin,
		client.TaskAwareRandom,
	}

	// Verify constants are different
	seen := make(map[client.LoadBalancerType]bool)
	for _, lbType := range types {
		if seen[lbType] {
			t.Errorf("Duplicate load balancer type value: %s", lbType)
		}
		seen[lbType] = true
	}
}

func TestLoadBalancerStatsStruct(t *testing.T) {
	now := time.Now()
	stats := client.LoadBalancerStats{
		TotalNodes:      10,
		ActiveNodes:     8,
		SelectionsCount: 100,
		LastSelection:   now,
		Strategy:        "round_robin",
	}

	if stats.TotalNodes != 10 {
		t.Errorf("Expected TotalNodes 10, got %d", stats.TotalNodes)
	}
	if stats.ActiveNodes != 8 {
		t.Errorf("Expected ActiveNodes 8, got %d", stats.ActiveNodes)
	}
	if stats.SelectionsCount != 100 {
		t.Errorf("Expected SelectionsCount 100, got %d", stats.SelectionsCount)
	}
	if !stats.LastSelection.Equal(now) {
		t.Errorf("Expected LastSelection %v, got %v", now, stats.LastSelection)
	}
	if stats.Strategy != "round_robin" {
		t.Errorf("Expected Strategy 'round_robin', got %s", stats.Strategy)
	}
}
