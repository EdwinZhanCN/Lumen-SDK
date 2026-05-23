package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/edwinzhancn/lumen-sdk/pkg/config"
	"github.com/edwinzhancn/lumen-sdk/pkg/utils"
	"go.uber.org/zap"
)

// HTTPDiscovery discovers nodes by polling lumenhubd's REST API.
type HTTPDiscovery struct {
	baseURL      string
	nodes        map[string]*NodeInfo
	scanInterval time.Duration
	watchers     []func([]*NodeInfo)

	client  *http.Client
	mu      sync.RWMutex
	ctx     context.Context
	cancel  context.CancelFunc
	running bool
	logger  *zap.Logger
	lastErr error
}

func NewHTTPDiscovery(cfg *config.DiscoveryConfig, logger *zap.Logger) *HTTPDiscovery {
	baseURL := ""
	scanInterval := 30 * time.Second
	if cfg != nil {
		baseURL = cfg.HubURL
		if cfg.ScanInterval > 0 {
			scanInterval = cfg.ScanInterval
		}
	}

	return &HTTPDiscovery{
		baseURL:      strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		nodes:        make(map[string]*NodeInfo),
		scanInterval: scanInterval,
		watchers:     make([]func([]*NodeInfo), 0),
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: ensureLogger(logger),
	}
}

func (h *HTTPDiscovery) Start(ctx context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.running {
		return utils.DiscoveryFailedError("discovery is already running")
	}
	if h.baseURL == "" {
		h.lastErr = fmt.Errorf("hub URL cannot be empty")
		return h.lastErr
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if h.cancel != nil {
		h.cancel()
	}
	h.ctx, h.cancel = context.WithCancel(ctx)
	h.running = true
	h.lastErr = nil

	go h.scanLoop(h.ctx)

	h.logger.Info("HTTP discovery started",
		zap.String("base_url", h.baseURL),
		zap.Duration("scan_interval", h.scanInterval),
	)
	return nil
}

func (h *HTTPDiscovery) Stop() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.running {
		h.lastErr = nil
		return nil
	}

	h.logger.Info("stopping HTTP discovery")
	if h.cancel != nil {
		h.cancel()
		h.cancel = nil
	}
	h.ctx = nil
	h.running = false
	nodeCount := len(h.nodes)
	h.nodes = make(map[string]*NodeInfo)
	h.watchers = make([]func([]*NodeInfo), 0)
	h.lastErr = nil

	h.logger.Info("HTTP discovery stopped successfully", zap.Int("nodes_cleared", nodeCount))
	return nil
}

func (h *HTTPDiscovery) GetNodes() []*NodeInfo {
	h.mu.RLock()
	defer h.mu.RUnlock()

	nodes := make([]*NodeInfo, 0, len(h.nodes))
	for _, node := range h.nodes {
		nodes = append(nodes, CloneNode(node))
	}
	return nodes
}

func (h *HTTPDiscovery) GetNode(id string) (*NodeInfo, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	node, exists := h.nodes[id]
	if !exists {
		return nil, false
	}
	return CloneNode(node), true
}

func (h *HTTPDiscovery) Watch(callback func([]*NodeInfo)) error {
	if callback == nil {
		return fmt.Errorf("callback cannot be nil")
	}

	h.mu.Lock()
	h.watchers = append(h.watchers, callback)
	nodes := cloneNodeMapValues(h.nodes)
	h.mu.Unlock()

	if len(nodes) > 0 {
		go callback(nodes)
	}
	return nil
}

func (h *HTTPDiscovery) Lookup(ctx context.Context, nodeID string) ([]string, error) {
	_ = ctx
	node, exists := h.GetNode(nodeID)
	if !exists || node == nil || strings.TrimSpace(node.Address) == "" {
		return nil, nil
	}
	return []string{node.Address}, nil
}

func (h *HTTPDiscovery) Error() error {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.lastErr
}

func (h *HTTPDiscovery) String() string {
	return "hubd"
}

func (h *HTTPDiscovery) Cache() map[string]CacheEntry {
	h.mu.RLock()
	defer h.mu.RUnlock()

	cache := make(map[string]CacheEntry, len(h.nodes))
	for id, node := range h.nodes {
		if node == nil {
			continue
		}
		cache[id] = CacheEntry{
			Addresses: UniqueTrimmedStrings([]string{node.Address}),
			When:      node.LastSeen,
			Found:     strings.TrimSpace(node.Address) != "",
		}
	}
	return cache
}

func (h *HTTPDiscovery) scanLoop(ctx context.Context) {
	h.performScan(ctx)

	ticker := time.NewTicker(h.scanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.performScan(ctx)
		}
	}
}

func (h *HTTPDiscovery) performScan(ctx context.Context) {
	nodes, err := h.fetchNodes(ctx)
	if err != nil {
		h.mu.Lock()
		h.lastErr = err
		h.mu.Unlock()
		h.logger.Warn("HTTP discovery scan failed", zap.Error(err), zap.String("base_url", h.baseURL))
		return
	}

	nodeMap := make(map[string]*NodeInfo, len(nodes))
	for _, node := range nodes {
		if node == nil || strings.TrimSpace(node.ID) == "" {
			continue
		}
		node = CloneNode(node)
		if node.LastSeen.IsZero() {
			node.LastSeen = time.Now()
		}
		node.InvalidateTaskCache()
		nodeMap[node.ID] = node
	}

	h.mu.Lock()
	h.nodes = nodeMap
	h.lastErr = nil
	h.mu.Unlock()

	h.notifyWatchers()
}

func (h *HTTPDiscovery) fetchNodes(ctx context.Context) ([]*NodeInfo, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	url := h.baseURL + "/v1/nodes"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("GET %s returned status %d", url, resp.StatusCode)
	}

	var raw json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	return decodeNodesResponse(raw)
}

func decodeNodesResponse(raw json.RawMessage) ([]*NodeInfo, error) {
	var nodes []*NodeInfo
	if err := json.Unmarshal(raw, &nodes); err == nil {
		return nodes, nil
	}

	var direct struct {
		Nodes []*NodeInfo `json:"nodes"`
	}
	if err := json.Unmarshal(raw, &direct); err == nil && direct.Nodes != nil {
		return direct.Nodes, nil
	}

	var envelope struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, err
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return nil, fmt.Errorf("nodes response missing data")
	}

	if err := json.Unmarshal(envelope.Data, &nodes); err == nil {
		return nodes, nil
	}

	var data struct {
		Nodes []*NodeInfo `json:"nodes"`
	}
	if err := json.Unmarshal(envelope.Data, &data); err != nil {
		return nil, err
	}
	return data.Nodes, nil
}

func (h *HTTPDiscovery) notifyWatchers() {
	h.mu.RLock()
	nodes := cloneNodeMapValues(h.nodes)
	watchers := make([]func([]*NodeInfo), len(h.watchers))
	copy(watchers, h.watchers)
	h.mu.RUnlock()

	for _, watcher := range watchers {
		go watcher(nodes)
	}
}

func cloneNodeMapValues(nodes map[string]*NodeInfo) []*NodeInfo {
	out := make([]*NodeInfo, 0, len(nodes))
	for _, node := range nodes {
		if node == nil {
			continue
		}
		out = append(out, CloneNode(node))
	}
	return out
}
