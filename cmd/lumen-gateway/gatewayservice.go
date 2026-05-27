package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/edwinzhancn/lumen-sdk/pkg/client"
	"github.com/edwinzhancn/lumen-sdk/pkg/config"
	"github.com/edwinzhancn/lumen-sdk/pkg/server/rest"
	"github.com/wailsapp/wails/v3/pkg/application"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// FrontendNodeInfo is a simplified DTO representation of a node for the React UI.
type FrontendNodeInfo struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Address string   `json:"address"`
	Status  string   `json:"status"`
	Tasks   []string `json:"tasks"`
}

// SavedConfig represents the settings persisted to disk.
type SavedConfig struct {
	Port         int    `yaml:"port"`
	ScanInterval string `yaml:"scan_interval"`
	HubURL       string `yaml:"hub_url"`
	LogLevel     string `yaml:"log_level"`
	Language     string `yaml:"language"`
}

// GatewayService bridges Wails IPC calls to the core Lumen SDK systems.
type GatewayService struct {
	cfg         *config.Config
	logger      *zap.Logger
	lumenClient *client.LumenClient
	router      *rest.Router
	startTime   time.Time
	mu          sync.RWMutex
	running     bool
	ctx         context.Context
}

// NewGatewayService creates a new instance of the Wails binding service.
func NewGatewayService() *GatewayService {
	return &GatewayService{}
}

// getConfigPath returns the path to the config file on disk.
func getConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	appDir := filepath.Join(dir, "Lumen Gateway")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(appDir, "config.yaml"), nil
}

// loadSavedConfig loads the persisted configuration from disk.
func (s *GatewayService) loadSavedConfig() (*SavedConfig, error) {
	path, err := getConfigPath()
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &SavedConfig{
			Port:         5866,
			ScanInterval: "30s",
			HubURL:       "",
			LogLevel:     "info",
			Language:     "zh",
		}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var sc SavedConfig
	if err := yaml.Unmarshal(data, &sc); err != nil {
		return nil, err
	}

	if sc.Port == 0 {
		sc.Port = 5866
	}
	if sc.ScanInterval == "" {
		sc.ScanInterval = "30s"
	}
	if sc.LogLevel == "" {
		sc.LogLevel = "info"
	}
	if sc.Language == "" {
		sc.Language = "zh"
	}

	return &sc, nil
}

// Start starts the Lumen client and REST API server.
func (s *GatewayService) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return nil
	}

	s.ctx = ctx

	logger, err := zap.NewDevelopment()
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}
	s.logger = logger

	sc, err := s.loadSavedConfig()
	if err != nil {
		s.logger.Error("Failed to load config from disk, using defaults", zap.Error(err))
		sc = &SavedConfig{
			Port:         5866,
			ScanInterval: "30s",
			LogLevel:     "info",
			Language:     "zh",
		}
	}

	cfg := config.DefaultConfig()

	cfg.Server.REST.Port = sc.Port
	dur, err := time.ParseDuration(sc.ScanInterval)
	if err == nil {
		cfg.Discovery.ScanInterval = dur
	}
	cfg.Discovery.HubURL = sc.HubURL
	cfg.Logging.Level = sc.LogLevel
	cfg.Discovery.MDNSEnabled = true // Force true

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid start configuration: %w", err)
	}
	s.cfg = cfg

	s.logger.Info("Starting Lumen Gateway Service...")

	lumenClient, err := client.NewLumenClient(s.cfg, s.logger)
	if err != nil {
		return fmt.Errorf("failed to create Lumen client: %w", err)
	}

	if err := lumenClient.Start(s.ctx); err != nil {
		return fmt.Errorf("failed to start Lumen client: %w", err)
	}
	s.lumenClient = lumenClient

	handler := rest.NewHandler(s.lumenClient, nil, s.logger)
	router := rest.NewRouter(handler, s.logger)
	router.SetupRoutes()
	s.router = router

	addr := fmt.Sprintf("%s:%d", s.cfg.Server.REST.Host, s.cfg.Server.REST.Port)
	go func() {
		s.logger.Info("REST server starting", zap.String("address", addr))
		if err := s.router.Start(addr); err != nil {
			s.logger.Error("REST server stopped with error", zap.Error(err))
		}
	}()

	s.startTime = time.Now()
	s.running = true
	s.logger.Info("Lumen Gateway Service started successfully")

	return nil
}

// Stop stops the client and REST server.
func (s *GatewayService) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	s.logger.Info("Stopping Lumen Gateway Service...")

	if s.router != nil {
		if err := s.router.ShutdownWithTimeout(3 * time.Second); err != nil {
			s.logger.Error("Failed to stop REST server", zap.Error(err))
		}
		s.router = nil
	}

	if s.lumenClient != nil {
		if err := s.lumenClient.Close(); err != nil {
			s.logger.Error("Failed to close Lumen client", zap.Error(err))
		}
		s.lumenClient = nil
	}

	s.running = false
	s.startTime = time.Time{}
	s.logger.Info("Lumen Gateway Service stopped")

	return nil
}

// SaveConfig saves the configuration to disk and triggers service hot-reload.
func (s *GatewayService) SaveConfig(cfg map[string]interface{}) error {
	valInt := func(k string, def int) int {
		if v, ok := cfg[k]; ok {
			switch val := v.(type) {
			case float64:
				return int(val)
			case int:
				return val
			}
		}
		return def
	}
	valStr := func(k string, def string) string {
		if v, ok := cfg[k]; ok {
			if str, ok := v.(string); ok {
				return str
			}
		}
		return def
	}

	sc := SavedConfig{
		Port:         valInt("port", 5866),
		ScanInterval: valStr("scanInterval", "30s"),
		HubURL:       valStr("hubUrl", ""),
		LogLevel:     valStr("logLevel", "info"),
		Language:     valStr("language", "zh"),
	}

	path, err := getConfigPath()
	if err != nil {
		return err
	}

	data, err := yaml.Marshal(&sc)
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return err
	}

	return s.Reload()
}

// Reload stops current services, reloads configuration from disk, and restarts services.
func (s *GatewayService) Reload() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	s.logger.Info("Config changed. Restarting services...")

	if s.router != nil {
		if err := s.router.ShutdownWithTimeout(3 * time.Second); err != nil {
			s.logger.Error("Failed to stop REST server during reload", zap.Error(err))
		}
		s.router = nil
	}

	if s.lumenClient != nil {
		if err := s.lumenClient.Close(); err != nil {
			s.logger.Error("Failed to close Lumen client during reload", zap.Error(err))
		}
		s.lumenClient = nil
	}

	s.running = false
	s.startTime = time.Time{}

	sc, err := s.loadSavedConfig()
	if err != nil {
		return fmt.Errorf("failed to load saved config during reload: %w", err)
	}

	cfg := config.DefaultConfig()

	cfg.Server.REST.Port = sc.Port
	dur, err := time.ParseDuration(sc.ScanInterval)
	if err == nil {
		cfg.Discovery.ScanInterval = dur
	}
	cfg.Discovery.HubURL = sc.HubURL
	cfg.Logging.Level = sc.LogLevel
	cfg.Discovery.MDNSEnabled = true // Force true

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid reload config: %w", err)
	}

	s.cfg = cfg

	s.logger.Info("Starting services with new config...")
	lumenClient, err := client.NewLumenClient(s.cfg, s.logger)
	if err != nil {
		return fmt.Errorf("failed to recreate client during reload: %w", err)
	}

	if err := lumenClient.Start(s.ctx); err != nil {
		return fmt.Errorf("failed to restart client: %w", err)
	}
	s.lumenClient = lumenClient

	handler := rest.NewHandler(s.lumenClient, nil, s.logger)
	router := rest.NewRouter(handler, s.logger)
	router.SetupRoutes()
	s.router = router

	addr := fmt.Sprintf("%s:%d", s.cfg.Server.REST.Host, s.cfg.Server.REST.Port)
	go func() {
		if err := s.router.Start(addr); err != nil {
			s.logger.Error("REST server stopped with error during reload", zap.Error(err))
		}
	}()

	s.startTime = time.Now()
	s.running = true
	s.logger.Info("Lumen Gateway Service successfully reloaded")

	return nil
}

// GetStatus returns the running status, uptime, and node statistics.
func (s *GatewayService) GetStatus() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	uptime := "0s"
	if !s.startTime.IsZero() {
		uptime = time.Since(s.startTime).Round(time.Second).String()
	}

	var totalNodes, activeNodes int
	if s.lumenClient != nil {
		nodes := s.lumenClient.GetNodes()
		totalNodes = len(nodes)
		for _, node := range nodes {
			if node.IsActive() {
				activeNodes++
			}
		}
	}

	language := "zh"
	sc, err := s.loadSavedConfig()
	if err == nil {
		language = sc.Language
	}

	return map[string]interface{}{
		"running":     s.running,
		"uptime":      uptime,
		"totalNodes":  totalNodes,
		"activeNodes": activeNodes,
		"port":        5866,
		"version":     "1.0.0",
		"language":    language,
	}
}

// GetNodes retrieves list of discovered nodes converted to UI DTOs.
func (s *GatewayService) GetNodes() []FrontendNodeInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.lumenClient == nil {
		return nil
	}

	nodes := s.lumenClient.GetNodes()
	res := make([]FrontendNodeInfo, 0, len(nodes))

	for _, node := range nodes {
		tasks := make([]string, 0, len(node.Tasks))
		for _, t := range node.Tasks {
			tasks = append(tasks, t.Name)
		}

		res = append(res, FrontendNodeInfo{
			ID:      node.ID,
			Name:    node.Name,
			Address: node.Address,
			Status:  string(node.Status),
			Tasks:   tasks,
		})
	}

	return res
}

// GetMetrics returns general metrics of the gateway client.
func (s *GatewayService) GetMetrics() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.lumenClient == nil {
		return map[string]interface{}{
			"totalReqs":   int64(0),
			"successReqs": int64(0),
			"failedReqs":  int64(0),
			"avgLatency":  0.0,
			"errorRate":   0.0,
			"activeNodes": 0,
			"totalNodes":  0,
		}
	}

	metrics := s.lumenClient.GetMetrics()
	uptime := time.Since(s.startTime).Seconds()
	qps := 0.0
	if uptime > 0 {
		qps = float64(metrics.TotalRequests) / uptime
	}

	return map[string]interface{}{
		"qps":         qps,
		"totalReqs":   metrics.TotalRequests,
		"successReqs": metrics.SuccessRequests,
		"failedReqs":  metrics.FailedRequests,
		"avgLatency":  float64(metrics.AverageLatency) / 1_000_000.0,
		"errorRate":   metrics.ErrorRate * 100.0,
		"activeNodes": metrics.ActiveNodes,
		"totalNodes":  metrics.TotalNodes,
	}
}

// GetTasks lists all tasks grouped by service.
func (s *GatewayService) GetTasks() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.lumenClient == nil {
		return nil
	}

	nodes := s.lumenClient.GetNodes()

	type TaskSummary struct {
		Name        string   `json:"name"`
		InputMimes  []string `json:"input_mimes,omitempty"`
		OutputMimes []string `json:"output_mimes,omitempty"`
		NodeID      string   `json:"node_id"`
		NodeName    string   `json:"node_name"`
	}

	serviceTasks := make(map[string][]TaskSummary)

	for _, node := range nodes {
		if !node.IsActive() {
			continue
		}

		for _, task := range node.Tasks {
			summary := TaskSummary{
				Name:        task.Name,
				InputMimes:  task.InputMimes,
				OutputMimes: task.OutputMimes,
				NodeID:      node.ID,
				NodeName:    node.Name,
			}

			serviceName := "unknown"
			for _, capability := range node.Capabilities {
				for _, capabilityTask := range capability.Tasks {
					if capabilityTask.Name == task.Name {
						serviceName = capability.ServiceName
						break
					}
				}
			}

			serviceTasks[serviceName] = append(serviceTasks[serviceName], summary)
		}
	}

	res := make(map[string]interface{})
	for k, v := range serviceTasks {
		res[k] = v
	}
	return res
}

// GetConfig returns the current configuration details.
func (s *GatewayService) GetConfig() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.cfg == nil {
		return nil
	}

	language := "zh"
	sc, err := s.loadSavedConfig()
	if err == nil {
		language = sc.Language
	}

	return map[string]interface{}{
		"restPort":     s.cfg.Server.REST.Port,
		"restHost":     s.cfg.Server.REST.Host,
		"scanInterval": s.cfg.Discovery.ScanInterval.String(),
		"hubUrl":       s.cfg.Discovery.HubURL,
		"logLevel":     s.cfg.Logging.Level,
		"language":     language,
	}
}

// Quit stops the gateway core and exits the application cleanly.
func (s *GatewayService) Quit() {
	s.Stop()
	application.Get().Quit()
}
