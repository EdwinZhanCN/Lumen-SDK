// Package client provides the core client implementation for the Lumen SDK.
//
// The client package is the main entry point for applications using Lumen ML services.
// It handles all aspects of distributed ML inference including:
//
//   - Service discovery via mDNS to find ML nodes automatically
//   - Connection pooling for efficient resource utilization
//   - Load balancing with multiple strategies (round-robin, weighted, task-aware)
//   - Automatic payload chunking for large data transfers
//   - Health monitoring and failover capabilities
//   - Metrics collection and observability
//
// # Key Components
//
// LumenClient is the main client type that orchestrates all operations:
//
//	cfg := config.DefaultConfig()
//	logger, _ := zap.NewProduction()
//	client, err := client.NewLumenClient(cfg, logger)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	ctx := context.Background()
//	client.Start(ctx)
//	defer client.Close()
//
// Load balancers distribute requests across available nodes:
//
//	balancer := client.CreateLoadBalancer(
//	    client.RoundRobin,
//	    &cfg.LoadBalancer,
//	    logger,
//	)
//
// # Architecture
//
// The client implements a distributed architecture:
//
//	Application
//	     ↓
//	LumenClient (orchestrates)
//	     ↓
//	├─ MDNSDiscovery (finds nodes)
//	├─ LoadBalancer (selects nodes)
//	├─ ConnectionPool (manages connections)
//	└─ Chunker (handles large payloads)
//	     ↓
//	ML Nodes (perform inference)
//
// # Usage Examples
//
// Basic text embedding:
//
//	text := []byte("Machine learning")
//	embReq, _ := types.NewEmbeddingRequest(text)
//	inferReq := types.NewInferRequest("text_embedding").
//	    ForEmbedding(embReq, "text_embedding").
//	    Build()
//	result, err := client.Infer(ctx, inferReq)
//
// Streaming inference:
//
//	respChan, err := client.InferStream(ctx, inferReq)
//	for resp := range respChan {
//	    if resp.IsFinal {
//	        // Process final result
//	    }
//	}
//
// # Role in Project
//
// The client package is the SDK's core, providing the primary interface for
// applications to interact with the Lumen ML platform. It abstracts the
// complexity of distributed systems, allowing developers to focus on ML
// operations rather than infrastructure management.
package client
