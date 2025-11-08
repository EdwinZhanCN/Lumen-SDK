package utils

import (
	"context"
	"math"
	"strings"
	"time"

	"github.com/sethvargo/go-retry"
)

// RetryConfig defines configuration for retry behavior with exponential backoff.
//
// Retry mechanisms help handle transient failures in distributed systems like
// network timeouts, temporary service unavailability, and connection issues.
//
// Role in project: Provides resilience against temporary failures in ML inference
// requests, making the SDK more robust in production environments.
//
// Example:
//
//	retryConfig := &utils.RetryConfig{
//	    Enabled:     true,
//	    MaxAttempts: 3,
//	    Backoff:     100 * time.Millisecond,
//	    MaxBackoff:  5 * time.Second,
//	    Multiplier:  2.0,
//	}
type RetryConfig struct {
	Enabled     bool          `json:"enabled"`
	MaxAttempts int           `json:"max_attempts"`
	Backoff     time.Duration `json:"backoff"`
	MaxBackoff  time.Duration `json:"max_backoff"`
	Multiplier  float64       `json:"multiplier"`
}

// DefaultRetryConfig 默认重试配置
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		Enabled:     true,
		MaxAttempts: 3,
		Backoff:     100 * time.Millisecond,
		MaxBackoff:  5 * time.Second,
		Multiplier:  2.0,
	}
}

// RetryFunc is the function signature for operations that can be retried.
//
// Functions of this type should be idempotent or handle their own state to
// ensure correct behavior when executed multiple times.
type RetryFunc func(ctx context.Context) error

// RetryableError is an interface for errors that indicate whether they should be retried.
//
// Implement this interface on custom error types to control retry behavior.
// The SDK automatically determines retry eligibility for common error types.
//
// Role in project: Enables intelligent retry decisions based on error semantics,
// preventing unnecessary retries for non-transient errors (e.g., validation errors).
type RetryableError interface {
	ShouldRetry() bool
}

// retryableError 可重试错误实现
type retryableError struct {
	err         error
	shouldRetry bool
}

func (r *retryableError) Error() string {
	return r.err.Error()
}

func (r *retryableError) ShouldRetry() bool {
	return r.shouldRetry
}

func (r *retryableError) Unwrap() error {
	return r.err
}

// NewRetryableError wraps an error with retry eligibility information.
//
// Use this to explicitly mark errors as retryable or non-retryable when the
// automatic detection isn't sufficient for your use case.
//
// Parameters:
//   - err: The underlying error
//   - shouldRetry: Whether this error should trigger a retry
//
// Returns:
//   - error: Wrapped error implementing RetryableError interface
//
// Example:
//
//	if networkErr != nil {
//	    // Mark as retryable
//	    return utils.NewRetryableError(networkErr, true)
//	}
//	if validationErr != nil {
//	    // Mark as non-retryable
//	    return utils.NewRetryableError(validationErr, false)
//	}
func NewRetryableError(err error, shouldRetry bool) error {
	if err == nil {
		return nil
	}
	return &retryableError{
		err:         err,
		shouldRetry: shouldRetry,
	}
}

// IsRetryable 检查错误是否可重试
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// 检查是否实现了RetryableError接口
	if re, ok := err.(RetryableError); ok {
		return re.ShouldRetry()
	}

	// 检查Lumen错误码
	if lumErr, ok := GetLumenError(err); ok {
		switch lumErr.Code {
		case ErrCodeTimeout, ErrCodeUnavailable, ErrCodeConnectionFailed:
			return true
		case ErrCodeInternal, ErrCodeInvalid, ErrCodeUnauthorized, ErrCodeForbidden:
			return false
		default:
			// 默认情况下，网络相关错误可以重试
			return isNetworkError(lumErr.Code)
		}
	}

	// 默认策略：网络超时和连接错误可重试
	return isNetworkErrorString(err.Error())
}

// isNetworkError 检查是否为网络相关错误
func isNetworkError(code ErrorCode) bool {
	switch code {
	case ErrCodeTimeout, ErrCodeUnavailable, ErrCodeConnectionFailed, ErrCodeServiceUnavailable:
		return true
	default:
		return false
	}
}

// isNetworkErrorString 检查错误字符串是否包含网络错误标识
func isNetworkErrorString(errStr string) bool {
	networkKeywords := []string{
		"timeout", "connection refused", "network unreachable",
		"connection reset", "connection timed out", "dns",
	}

	lowerErr := strings.ToLower(errStr)
	for _, keyword := range networkKeywords {
		if strings.Contains(lowerErr, keyword) {
			return true
		}
	}
	return false
}

// Retry executes a function with automatic retry on transient failures.
//
// This function implements exponential backoff retry logic with configurable
// parameters. It automatically determines if errors are retryable based on:
//   - RetryableError interface implementation
//   - Lumen error codes (timeout, unavailable, connection failed)
//   - Common network error patterns
//
// Non-retryable errors (validation, authorization, etc.) fail immediately without retry.
//
// Parameters:
//   - ctx: Context for cancellation (respects context timeout/cancellation)
//   - config: Retry configuration (attempts, backoff, etc.)
//   - fn: The function to execute with retry logic
//
// Returns:
//   - error: The last error encountered, or nil if successful
//
// Role in project: Adds resilience to ML inference operations by handling transient
// network and service failures automatically. Essential for production reliability.
//
// Example:
//
//	retryConfig := utils.DefaultRetryConfig()
//	err := utils.Retry(ctx, retryConfig, func(ctx context.Context) error {
//	    result, err := client.Infer(ctx, request)
//	    if err != nil {
//	        return err
//	    }
//	    // Process result
//	    return nil
//	})
//	if err != nil {
//	    log.Printf("Failed after retries: %v", err)
//	}
func Retry(ctx context.Context, config *RetryConfig, fn RetryFunc) error {
	if !config.Enabled {
		return fn(ctx)
	}

	// 创建重试策略
	policy := retry.NewExponential(config.Backoff)
	policy = retry.WithMaxRetries(uint64(config.MaxAttempts-1), policy)
	policy = retry.WithMaxDuration(config.MaxBackoff, policy)

	var lastErr error
	err := retry.Do(ctx, policy, func(ctx context.Context) error {
		err := fn(ctx)
		if err != nil {
			lastErr = err
			if IsRetryable(err) {
				return retry.RetryableError(err)
			}
			return err // 不可重试的错误直接返回
		}
		return nil
	})

	if err != nil {
		// 如果是最后一次重试失败，返回原始错误
		if lastErr != nil {
			return lastErr
		}
		return err
	}

	return nil
}

// RetryWithCallback 带回调的重试执行
type RetryCallback func(attempt int, err error)

func RetryWithCallback(ctx context.Context, config *RetryConfig, fn RetryFunc, callback RetryCallback) error {
	if !config.Enabled {
		return fn(ctx)
	}

	attempt := 0
	policy := retry.NewExponential(config.Backoff)
	policy = retry.WithMaxRetries(uint64(config.MaxAttempts-1), policy)
	policy = retry.WithMaxDuration(config.MaxBackoff, policy)

	var lastErr error
	err := retry.Do(ctx, policy, func(ctx context.Context) error {
		attempt++
		err := fn(ctx)
		if err != nil {
			lastErr = err
			if callback != nil {
				callback(attempt, err)
			}
			if IsRetryable(err) {
				return retry.RetryableError(err)
			}
			return err
		}
		return nil
	})

	if err != nil {
		if lastErr != nil {
			return lastErr
		}
		return err
	}

	return nil
}

// Backoff 计算退避时间
func Backoff(attempt int, base time.Duration, multiplier float64, max time.Duration) time.Duration {
	if attempt <= 0 {
		return base
	}

	backoff := time.Duration(float64(base) * math.Pow(multiplier, float64(attempt-1)))
	if backoff > max {
		backoff = max
	}

	return backoff
}

// CircuitBreaker implements the circuit breaker pattern for fault tolerance.
//
// The circuit breaker prevents cascading failures by:
//   - Tracking consecutive failures
//   - Opening the circuit after threshold failures (rejecting requests immediately)
//   - Allowing test requests after a reset timeout (half-open state)
//   - Closing the circuit when requests succeed again
//
// States:
//   - Closed: Normal operation, all requests go through
//   - Open: Circuit tripped, requests fail fast without execution
//   - Half-Open: Testing if service recovered, limited requests allowed
//
// Role in project: Protects the system from repeatedly calling failing services,
// allowing them time to recover and preventing resource exhaustion.
//
// Example:
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
type CircuitBreaker struct {
	name        string
	maxFailures int
	resetTime   time.Duration
	failures    int
	lastFailure time.Time
	state       CircuitState
}

type CircuitState int

const (
	CircuitClosed CircuitState = iota
	CircuitOpen
	CircuitHalfOpen
)

// NewCircuitBreaker 创建断路器
func NewCircuitBreaker(name string, maxFailures int, resetTime time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		name:        name,
		maxFailures: maxFailures,
		resetTime:   resetTime,
		state:       CircuitClosed,
	}
}

// Execute 执行函数
func (cb *CircuitBreaker) Execute(fn func() error) error {
	if cb.state == CircuitOpen {
		if time.Since(cb.lastFailure) > cb.resetTime {
			cb.state = CircuitHalfOpen
		} else {
			return UnavailableError("circuit breaker is open", map[string]interface{}{
				"breaker": cb.name,
				"state":   "open",
			})
		}
	}

	err := fn()
	if err != nil {
		cb.onFailure()
		return err
	}

	cb.onSuccess()
	return nil
}

// onSuccess 成功时调用
func (cb *CircuitBreaker) onSuccess() {
	cb.failures = 0
	if cb.state == CircuitHalfOpen {
		cb.state = CircuitClosed
	}
}

// onFailure 失败时调用
func (cb *CircuitBreaker) onFailure() {
	cb.failures++
	cb.lastFailure = time.Now()

	if cb.failures >= cb.maxFailures {
		cb.state = CircuitOpen
	}
}

// GetState 获取断路器状态
func (cb *CircuitBreaker) GetState() CircuitState {
	return cb.state
}

// Reset 重置断路器
func (cb *CircuitBreaker) Reset() {
	cb.failures = 0
	cb.state = CircuitClosed
	cb.lastFailure = time.Time{}
}
