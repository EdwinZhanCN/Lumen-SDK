package utils

import (
	"fmt"
	"strings"
)

// ErrorCode represents a standardized error code for the Lumen SDK.
//
// Error codes provide a machine-readable way to identify error types,
// enabling clients to handle errors programmatically with appropriate
// retry logic, user messages, and recovery strategies.
//
// Role in project: Standardizes error handling across the SDK, enabling
// consistent error reporting and intelligent error recovery.
type ErrorCode string

const (
	// General error codes applicable across all components
	ErrCodeInternal     ErrorCode = "INTERNAL"     // Internal server/SDK error
	ErrCodeInvalid      ErrorCode = "INVALID"
	ErrCodeTimeout      ErrorCode = "TIMEOUT"
	ErrCodeUnavailable  ErrorCode = "UNAVAILABLE"
	ErrCodeNotFound     ErrorCode = "NOT_FOUND"
	ErrCodeUnauthorized ErrorCode = "UNAUTHORIZED"
	ErrCodeForbidden    ErrorCode = "FORBIDDEN"

	// Lumen特定错误码
	ErrCodeNodeNotFound       ErrorCode = "NODE_NOT_FOUND"
	ErrCodeServiceUnavailable ErrorCode = "SERVICE_UNAVAILABLE"
	ErrCodeTaskUnsupported    ErrorCode = "TASK_UNSUPPORTED"
	ErrCodecMismatch          ErrorCode = "CODEC_MISMATCH"
	ErrCodeDiscoveryFailed    ErrorCode = "DISCOVERY_FAILED"
	ErrCodeConnectionFailed   ErrorCode = "CONNECTION_FAILED"
	ErrCodeRequestFailed      ErrorCode = "REQUEST_FAILED"
	ErrCodeResponseFailed     ErrorCode = "RESPONSE_FAILED"
)

// LumenError represents a structured error from the Lumen SDK.
//
// This error type provides:
//   - Standardized error codes for programmatic handling
//   - Human-readable error messages
//   - Optional structured details (can be logged or returned to API clients)
//   - Error wrapping/chaining support via Cause
//
// Role in project: Provides structured, actionable error information throughout
// the SDK. Essential for debugging, monitoring, and building resilient applications.
//
// Example:
//
//	err := utils.NodeNotFoundError("node-123", map[string]interface{}{
//	    "requested_at": time.Now(),
//	    "available_nodes": 5,
//	})
//	if utils.HasErrorCode(err, utils.ErrCodeNodeNotFound) {
//	    // Handle node not found specifically
//	}
type LumenError struct {
	Code    ErrorCode   `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
	Cause   error       `json:"-"`
}

// Error 实现error接口
func (e *LumenError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap 支持错误链
func (e *LumenError) Unwrap() error {
	return e.Cause
}

// NewLumenError creates a new LumenError with the specified code and message.
//
// Parameters:
//   - code: Standardized error code (e.g., ErrCodeTimeout)
//   - message: Human-readable error description
//   - details: Optional structured details (first element used if provided)
//
// Returns:
//   - *LumenError: New error instance
//
// Example:
//
//	err := utils.NewLumenError(
//	    utils.ErrCodeTimeout,
//	    "inference request timed out",
//	    map[string]interface{}{"timeout": "30s", "node": "node-1"},
//	)
func NewLumenError(code ErrorCode, message string, details ...interface{}) *LumenError {
	err := &LumenError{
		Code:    code,
		Message: message,
	}

	if len(details) > 0 {
		err.Details = details[0]
	}

	return err
}

// Wrap wraps an existing error with additional context and a Lumen error code.
//
// This function creates a new LumenError that preserves the original error
// as the cause, enabling error chain unwrapping with errors.Unwrap().
//
// Parameters:
//   - err: The original error to wrap
//   - code: Lumen error code for categorization
//   - message: Additional context message
//   - details: Optional structured details
//
// Returns:
//   - *LumenError: Wrapped error with Lumen context
//
// Example:
//
//	_, err := conn.Dial(address)
//	if err != nil {
//	    return utils.Wrap(err, utils.ErrCodeConnectionFailed,
//	        "failed to connect to ML node",
//	        map[string]string{"address": address})
//	}
func Wrap(err error, code ErrorCode, message string, details ...interface{}) *LumenError {
	return &LumenError{
		Code:    code,
		Message: message,
		Details: getFirst(details),
		Cause:   err,
	}
}

// getFirst 获取第一个非空值
func getFirst(details []interface{}) interface{} {
	for _, d := range details {
		if d != nil {
			return d
		}
	}
	return nil
}

// IsLumenError 检查是否为Lumen错误
func IsLumenError(err error) bool {
	_, ok := err.(*LumenError)
	return ok
}

// GetLumenError 获取Lumen错误
func GetLumenError(err error) (*LumenError, bool) {
	if lumErr, ok := err.(*LumenError); ok {
		return lumErr, true
	}
	return nil, false
}

// HasErrorCode 检查是否包含特定错误码
func HasErrorCode(err error, code ErrorCode) bool {
	if lumErr, ok := GetLumenError(err); ok {
		return lumErr.Code == code
	}
	return false
}

// InternalError creates an internal error indicating an unexpected condition.
//
// Use for unexpected errors, programming errors, or unhandled edge cases.
// These typically indicate bugs or misconfigurations.
//
// Example:
//
//	if node == nil {
//	    return utils.InternalError("node should never be nil at this point")
//	}
func InternalError(message string, details ...interface{}) *LumenError {
	return NewLumenError(ErrCodeInternal, message, details...)
}

func InvalidError(message string, details ...interface{}) *LumenError {
	return NewLumenError(ErrCodeInvalid, message, details...)
}

func TimeoutError(message string, details ...interface{}) *LumenError {
	return NewLumenError(ErrCodeTimeout, message, details...)
}

func UnavailableError(message string, details ...interface{}) *LumenError {
	return NewLumenError(ErrCodeUnavailable, message, details...)
}

func NotFoundError(message string, details ...interface{}) *LumenError {
	return NewLumenError(ErrCodeNotFound, message, details...)
}

func UnauthorizedError(message string, details ...interface{}) *LumenError {
	return NewLumenError(ErrCodeUnauthorized, message, details...)
}

func ForbiddenError(message string, details ...interface{}) *LumenError {
	return NewLumenError(ErrCodeForbidden, message, details...)
}

// NodeNotFoundError creates an error indicating a requested ML node was not found.
//
// This error typically occurs when:
//   - The node ID doesn't exist in the discovered nodes
//   - The node has been removed or gone offline
//   - Service discovery hasn't found any nodes yet
//
// Parameters:
//   - nodeID: The ID of the node that was not found
//   - details: Optional additional context
//
// Example:
//
//	node, exists := discovery.GetNode(nodeID)
//	if !exists {
//	    return utils.NodeNotFoundError(nodeID, map[string]interface{}{
//	        "available_nodes": discovery.GetNodeCount(),
//	    })
//	}
func NodeNotFoundError(nodeID string, details ...interface{}) *LumenError {
	return NewLumenError(ErrCodeNodeNotFound,
		fmt.Sprintf("node not found: %s", nodeID), details...)
}

func ServiceUnavailableError(service string, details ...interface{}) *LumenError {
	return NewLumenError(ErrCodeServiceUnavailable,
		fmt.Sprintf("service unavailable: %s", service), details...)
}

func TaskUnsupportedError(task string, details ...interface{}) *LumenError {
	return NewLumenError(ErrCodeTaskUnsupported,
		fmt.Sprintf("task not supported: %s", task), details...)
}

func CodecMismatchError(expected, actual string, details ...interface{}) *LumenError {
	return NewLumenError(ErrCodecMismatch,
		fmt.Sprintf("codec mismatch: expected=%s, actual=%s", expected, actual), details...)
}

func DiscoveryFailedError(message string, details ...interface{}) *LumenError {
	return NewLumenError(ErrCodeDiscoveryFailed,
		fmt.Sprintf("discovery failed: %s", message), details...)
}

func ConnectionFailedError(target string, details ...interface{}) *LumenError {
	return NewLumenError(ErrCodeConnectionFailed,
		fmt.Sprintf("connection failed: %s", target), details...)
}

func RequestFailedError(message string, details ...interface{}) *LumenError {
	return NewLumenError(ErrCodeRequestFailed,
		fmt.Sprintf("request failed: %s", message), details...)
}

func ResponseFailedError(message string, details ...interface{}) *LumenError {
	return NewLumenError(ErrCodeResponseFailed,
		fmt.Sprintf("response failed: %s", message), details...)
}

// ErrorAggregator collects multiple errors for batch operations.
//
// Use this when performing operations on multiple items where you want to:
//   - Continue processing despite individual failures
//   - Report all errors at once rather than failing on first error
//   - Track partial success/failure in batch operations
//
// Role in project: Enables robust error handling in batch operations like
// initializing multiple connections or processing multiple inference requests.
//
// Example:
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
type ErrorAggregator struct {
	errors []error
}

// NewErrorAggregator 创建错误聚合器
func NewErrorAggregator() *ErrorAggregator {
	return &ErrorAggregator{
		errors: make([]error, 0),
	}
}

// Add 添加错误
func (ea *ErrorAggregator) Add(err error) {
	if err != nil {
		ea.errors = append(ea.errors, err)
	}
}

// HasErrors 是否有错误
func (ea *ErrorAggregator) HasErrors() bool {
	return len(ea.errors) > 0
}

// GetErrors 获取所有错误
func (ea *ErrorAggregator) GetErrors() []error {
	return ea.errors
}

// Error 返回聚合错误信息
func (ea *ErrorAggregator) Error() string {
	if len(ea.errors) == 0 {
		return ""
	}

	if len(ea.errors) == 1 {
		return ea.errors[0].Error()
	}

	var messages []string
	for _, err := range ea.errors {
		messages = append(messages, err.Error())
	}

	return fmt.Sprintf("multiple errors occurred: %s", strings.Join(messages, "; "))
}

// ToError 转换为error接口
func (ea *ErrorAggregator) ToError() error {
	if !ea.HasErrors() {
		return nil
	}
	return ea
}

// Reset 重置错误聚合器
func (ea *ErrorAggregator) Reset() {
	ea.errors = ea.errors[:0]
}

// Recover 恢复panic并转换为错误
func Recover() error {
	if r := recover(); r != nil {
		switch v := r.(type) {
		case error:
			return InternalError("panic occurred", v)
		default:
			return InternalError("panic occurred", v)
		}
	}
	return nil
}

// SafeExecute 安全执行函数，捕获panic
func SafeExecute(fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			switch v := r.(type) {
			case error:
				err = InternalError("panic in safe execution", v)
			default:
				err = InternalError("panic in safe execution", v)
			}
		}
	}()

	return fn()
}
