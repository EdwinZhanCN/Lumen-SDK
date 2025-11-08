// Package utils provides utility functions and types for the Lumen SDK.
//
// The utils package contains common utilities used throughout the SDK:
//
//   - Structured error handling with LumenError
//   - Logging configuration and initialization
//   - Retry mechanisms with exponential backoff
//   - Circuit breaker for fault tolerance
//   - Health monitoring utilities
//
// # Structured Errors
//
// Use LumenError for consistent error handling:
//
//	if node == nil {
//	    return utils.NodeNotFoundError(nodeID, map[string]interface{}{
//	        "available_nodes": nodeCount,
//	    })
//	}
//
// Check error types programmatically:
//
//	if utils.HasErrorCode(err, utils.ErrCodeTimeout) {
//	    // Handle timeout specifically
//	    return retry(operation)
//	}
//
// # Logging
//
// Initialize structured logging:
//
//	logCfg := &config.LoggingConfig{
//	    Level:  "info",
//	    Format: "json",
//	    Output: "stdout",
//	}
//	utils.InitLogger(logCfg)
//
//	// Use the logger
//	utils.Logger.Info("Operation completed",
//	    zap.String("operation", "inference"),
//	    zap.Duration("duration", elapsed))
//
// # Retry Logic
//
// Add resilience with automatic retries:
//
//	retryConfig := utils.DefaultRetryConfig()
//	err := utils.Retry(ctx, retryConfig, func(ctx context.Context) error {
//	    result, err := client.Infer(ctx, request)
//	    if err != nil {
//	        return err
//	    }
//	    return processResult(result)
//	})
//
// The retry mechanism automatically determines if errors are retryable:
//   - Timeout errors: retried
//   - Network errors: retried
//   - Validation errors: not retried
//   - Authorization errors: not retried
//
// # Circuit Breaker
//
// Protect against cascading failures:
//
//	cb := utils.NewCircuitBreaker("ml-node-1", 5, 30*time.Second)
//	err := cb.Execute(func() error {
//	    return client.Infer(ctx, request)
//	})
//	if err != nil {
//	    if cb.GetState() == utils.CircuitOpen {
//	        log.Println("Circuit breaker open, service unavailable")
//	    }
//	}
//
// # Error Aggregation
//
// Collect multiple errors in batch operations:
//
//	aggr := utils.NewErrorAggregator()
//	for _, node := range nodes {
//	    if err := node.Connect(); err != nil {
//	        aggr.Add(err)
//	    }
//	}
//	if aggr.HasErrors() {
//	    log.Printf("Connection errors: %v", aggr.Error())
//	}
//
// # Role in Project
//
// The utils package provides foundational utilities that enhance the SDK's
// reliability, observability, and error handling. These utilities are used
// throughout the codebase to ensure consistent behavior and robust operation.
package utils
