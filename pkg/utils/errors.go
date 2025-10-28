package utils

import (
	"fmt"
	"strings"
)

// ErrorCode 错误码类型
type ErrorCode string

const (
	// 通用错误码
	ErrCodeInternal     ErrorCode = "INTERNAL"
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

// LumenError Lumen SDK错误结构
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

// NewLumenError 创建新的Lumen错误
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

// Wrap 包装已有错误
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

// 错误构造函数
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

// Lumen特定错误构造函数
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

// ErrorAggregator 错误聚合器
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
