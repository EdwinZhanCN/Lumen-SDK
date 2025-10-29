package utils

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	pb "github.com/edwinzhancn/lumen-sdk/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

// HealthStatus 健康状态
type HealthStatus string

const (
	StatusHealthy   HealthStatus = "healthy"
	StatusUnhealthy HealthStatus = "unhealthy"
	StatusUnknown   HealthStatus = "unknown"
)

// HealthCheckResult 健康检查结果
type HealthCheckResult struct {
	Status    HealthStatus           `json:"status"`
	Message   string                 `json:"message"`
	Timestamp time.Time              `json:"timestamp"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Duration  time.Duration          `json:"duration"`
}

// HealthChecker 健康检查器接口
type HealthChecker interface {
	Check(ctx context.Context) *HealthCheckResult
	Name() string
}

// GRPCHealthChecker gRPC健康检查器
type GRPCHealthChecker struct {
	client  pb.InferenceClient
	name    string
	timeout time.Duration
}

// NewGRPCHealthChecker 创建gRPC健康检查器
func NewGRPCHealthChecker(client pb.InferenceClient, name string, timeout time.Duration) *GRPCHealthChecker {
	return &GRPCHealthChecker{
		client:  client,
		name:    name,
		timeout: timeout,
	}
}

// Name 返回检查器名称
func (h *GRPCHealthChecker) Name() string {
	return h.name
}

// Check 执行健康检查
func (h *GRPCHealthChecker) Check(ctx context.Context) *HealthCheckResult {
	start := time.Now()

	// 设置超时
	timeoutCtx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()

	result := &HealthCheckResult{
		Timestamp: start,
		Details:   make(map[string]interface{}),
	}

	// 执行健康检查
	_, err := h.client.Health(timeoutCtx, &emptypb.Empty{})
	result.Duration = time.Since(start)

	if err != nil {
		result.Status = StatusUnhealthy
		result.Message = err.Error()
		result.Details["error"] = err.Error()
	} else {
		result.Status = StatusHealthy
		result.Message = "service is healthy"
		result.Details["response_time"] = result.Duration.String()
	}

	return result
}

// HTTPHealthChecker HTTP健康检查器
type HTTPHealthChecker struct {
	url     string
	name    string
	timeout time.Duration
	client  *http.Client
}

// NewHTTPHealthChecker 创建HTTP健康检查器
func NewHTTPHealthChecker(url, name string, timeout time.Duration) *HTTPHealthChecker {
	return &HTTPHealthChecker{
		url:     url,
		name:    name,
		timeout: timeout,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// Name 返回检查器名称
func (h *HTTPHealthChecker) Name() string {
	return h.name
}

// Check 执行健康检查
func (h *HTTPHealthChecker) Check(ctx context.Context) *HealthCheckResult {
	start := time.Now()

	result := &HealthCheckResult{
		Timestamp: start,
		Details:   make(map[string]interface{}),
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "GET", h.url, nil)
	if err != nil {
		result.Status = StatusUnhealthy
		result.Message = "failed to create request: " + err.Error()
		result.Duration = time.Since(start)
		return result
	}

	// 执行请求
	resp, err := h.client.Do(req)
	result.Duration = time.Since(start)

	if err != nil {
		result.Status = StatusUnhealthy
		result.Message = err.Error()
		result.Details["error"] = err.Error()
		return result
	}
	defer resp.Body.Close()

	// 检查状态码
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		result.Status = StatusHealthy
		result.Message = "HTTP service is healthy"
		result.Details["status_code"] = resp.StatusCode
		result.Details["response_time"] = result.Duration.String()
	} else {
		result.Status = StatusUnhealthy
		result.Message = fmt.Sprintf("HTTP service returned status %d", resp.StatusCode)
		result.Details["status_code"] = resp.StatusCode
	}

	return result
}

// CompositeHealthChecker 复合健康检查器
type CompositeHealthChecker struct {
	name     string
	checkers []HealthChecker
}

// NewCompositeHealthChecker 创建复合健康检查器
func NewCompositeHealthChecker(name string, checkers ...HealthChecker) *CompositeHealthChecker {
	return &CompositeHealthChecker{
		name:     name,
		checkers: checkers,
	}
}

// Name 返回检查器名称
func (h *CompositeHealthChecker) Name() string {
	return h.name
}

// Check 执行复合健康检查
func (h *CompositeHealthChecker) Check(ctx context.Context) *HealthCheckResult {
	start := time.Now()

	result := &HealthCheckResult{
		Timestamp: start,
		Details:   make(map[string]interface{}),
	}

	allHealthy := true
	checkResults := make(map[string]*HealthCheckResult)

	for _, checker := range h.checkers {
		checkResult := checker.Check(ctx)
		checkResults[checker.Name()] = checkResult

		if checkResult.Status != StatusHealthy {
			allHealthy = false
		}
	}

	result.Duration = time.Since(start)

	if allHealthy {
		result.Status = StatusHealthy
		result.Message = "all health checks passed"
	} else {
		result.Status = StatusUnhealthy
		result.Message = "some health checks failed"
	}

	result.Details["checkers"] = checkResults
	result.Details["total_checks"] = len(h.checkers)
	result.Details["healthy_checks"] = countHealthyChecks(checkResults)

	return result
}

// countHealthyChecks 统计健康的检查器数量
func countHealthyChecks(results map[string]*HealthCheckResult) int {
	count := 0
	for _, result := range results {
		if result.Status == StatusHealthy {
			count++
		}
	}
	return count
}

// HealthMonitor 健康监控器
type HealthMonitor struct {
	checkers map[string]HealthChecker
	results  map[string]*HealthCheckResult
	mu       sync.RWMutex
	interval time.Duration
	stopCh   chan struct{}
	running  bool
}

// NewHealthMonitor 创建健康监控器
func NewHealthMonitor(interval time.Duration) *HealthMonitor {
	return &HealthMonitor{
		checkers: make(map[string]HealthChecker),
		results:  make(map[string]*HealthCheckResult),
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// AddChecker 添加健康检查器
func (hm *HealthMonitor) AddChecker(checker HealthChecker) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	hm.checkers[checker.Name()] = checker
}

// RemoveChecker 移除健康检查器
func (hm *HealthMonitor) RemoveChecker(name string) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	delete(hm.checkers, name)
	delete(hm.results, name)
}

// CheckAll 执行所有健康检查
func (hm *HealthMonitor) CheckAll(ctx context.Context) map[string]*HealthCheckResult {
	hm.mu.RLock()
	checkers := make(map[string]HealthChecker)
	for name, checker := range hm.checkers {
		checkers[name] = checker
	}
	hm.mu.RUnlock()

	results := make(map[string]*HealthCheckResult)
	var wg sync.WaitGroup

	for name, checker := range checkers {
		wg.Add(1)
		go func(name string, checker HealthChecker) {
			defer wg.Done()
			results[name] = checker.Check(ctx)
		}(name, checker)
	}

	wg.Wait()

	// 更新缓存结果
	hm.mu.Lock()
	for name, result := range results {
		hm.results[name] = result
	}
	hm.mu.Unlock()

	return results
}

// GetResult 获取特定检查器的最新结果
func (hm *HealthMonitor) GetResult(name string) (*HealthCheckResult, bool) {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	result, exists := hm.results[name]
	return result, exists
}

// GetAllResults 获取所有检查器的最新结果
func (hm *HealthMonitor) GetAllResults() map[string]*HealthCheckResult {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	results := make(map[string]*HealthCheckResult)
	for name, result := range hm.results {
		results[name] = result
	}
	return results
}

// Start 启动定期健康检查
func (hm *HealthMonitor) Start(ctx context.Context) {
	hm.mu.Lock()
	if hm.running {
		hm.mu.Unlock()
		return
	}
	hm.running = true
	hm.mu.Unlock()

	ticker := time.NewTicker(hm.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			hm.Stop()
			return
		case <-hm.stopCh:
			return
		case <-ticker.C:
			hm.CheckAll(ctx)
		}
	}
}

// Stop 停止健康监控
func (hm *HealthMonitor) Stop() {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	if hm.running {
		hm.running = false
		close(hm.stopCh)
		hm.stopCh = make(chan struct{})
	}
}

// IsRunning 检查是否正在运行
func (hm *HealthMonitor) IsRunning() bool {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	return hm.running
}

// GetHealthyCheckers 获取所有健康的检查器
func (hm *HealthMonitor) GetHealthyCheckers() []string {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	var healthy []string
	for name, result := range hm.results {
		if result.Status == StatusHealthy {
			healthy = append(healthy, name)
		}
	}
	return healthy
}

// GetUnhealthyCheckers 获取所有不健康的检查器
func (hm *HealthMonitor) GetUnhealthyCheckers() []string {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	var unhealthy []string
	for name, result := range hm.results {
		if result.Status == StatusUnhealthy {
			unhealthy = append(unhealthy, name)
		}
	}
	return unhealthy
}
