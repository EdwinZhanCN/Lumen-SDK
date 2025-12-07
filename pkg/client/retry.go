package client

import (
	"context"
	"fmt"
	"strings"
	"time"

	pb "github.com/edwinzhancn/lumen-sdk/proto"
)

// RetryOption configures retry behavior
type RetryOption func(*RetryConfig)

type RetryConfig struct {
	maxWaitTime     time.Duration
	retryInterval   time.Duration
	maxRetries      int
	waitForTask     bool
}

// WithMaxWaitTime sets maximum time to wait for successful inference
func WithMaxWaitTime(d time.Duration) RetryOption {
	return func(c *RetryConfig) {
		c.maxWaitTime = d
	}
}

// WithRetryInterval sets time between retry attempts
func WithRetryInterval(d time.Duration) RetryOption {
	return func(c *RetryConfig) {
		c.retryInterval = d
	}
}

// WithMaxRetries sets maximum number of retry attempts
func WithMaxRetries(n int) RetryOption {
	return func(c *RetryConfig) {
		c.maxRetries = n
	}
}

// WithWaitForTask enables waiting for task to become available
func WithWaitForTask(wait bool) RetryOption {
	return func(c *RetryConfig) {
		c.waitForTask = wait
	}
}

// defaultRetryConfig returns default retry settings
func defaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		maxWaitTime:   30 * time.Second,
		retryInterval: 2 * time.Second,
		maxRetries:    0, // unlimited until timeout
		waitForTask:   true,
	}
}

// InferWithRetry performs inference with automatic retry logic
func (c *LumenClient) InferWithRetry(ctx context.Context, req *pb.InferRequest, opts ...RetryOption) (*pb.InferResponse, error) {
	config := defaultRetryConfig()
	for _, opt := range opts {
		opt(config)
	}

	startTime := time.Now()
	retryCount := 0
	var lastError error

	for time.Since(startTime) < config.maxWaitTime {
		// Check max retries if specified
		if config.maxRetries > 0 && retryCount >= config.maxRetries {
			break
		}

		// If waiting for task, check if it's available
		if config.waitForTask && !c.IsTaskAvailable(req.Task) {
			fmt.Printf("⏳ Task '%s' not available yet, waiting... (%d)\n", req.Task, retryCount+1)
			time.Sleep(config.retryInterval)
			retryCount++
			continue
		}

		// Try inference
		resp, err := c.Infer(ctx, req)
		if err == nil {
			if retryCount > 0 {
				fmt.Printf("✅ Success after %d retries\n", retryCount)
			}
			return resp, nil
		}

		lastError = err

		// Check if error is retryable
		if !isRetryableError(err) {
			break
		}

		fmt.Printf("⚠️  Retry %d: %v\n", retryCount+1, err)
		retryCount++
		time.Sleep(config.retryInterval)
	}

	if lastError != nil {
		return nil, fmt.Errorf("failed after %d retries: %w", retryCount, lastError)
	}
	return nil, fmt.Errorf("timeout after %v", config.maxWaitTime)
}

// IsTaskAvailable checks if a task is available on any active node
func (c *LumenClient) IsTaskAvailable(taskName string) bool {
	nodes := c.GetNodes()
	for _, node := range nodes {
		if node.IsActive() {
			for _, task := range node.Tasks {
				if task.Name == taskName {
					return true
				}
			}
		}
	}
	return false
}

// isRetryableError determines if an error should be retried
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Non-retryable errors
	nonRetryableErrors := []string{
		"invalid payload",
		"malformed request",
		"authentication failed",
		"permission denied",
		"parsing failed",
	}

	for _, nonRetryable := range nonRetryableErrors {
		if contains(errStr, nonRetryable) {
			return false
		}
	}

	// Retryable errors (network, timeouts, node issues, etc.)
	retryableErrors := []string{
		"no nodes",
		"node not found",
		"connection",
		"timeout",
		"unavailable",
		"temporary",
	}

	for _, retryable := range retryableErrors {
		if contains(errStr, retryable) {
			return true
		}
	}

	return false
}

func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}