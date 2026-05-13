package discovery

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// CacheEntry stores cached address results for a discovered node.
type CacheEntry struct {
	Addresses []string  `json:"addresses"`
	When      time.Time `json:"when"`
	Found     bool      `json:"found"`
}

// Finder is the transport-agnostic lookup contract for node discovery backends.
type Finder interface {
	Lookup(ctx context.Context, nodeID string) ([]string, error)
	Error() error
	String() string
	Cache() map[string]CacheEntry
}

// FinderService is a Finder with lifecycle hooks.
type FinderService interface {
	Finder
	Start(ctx context.Context) error
	Stop() error
}

// ServiceDiscovery is the higher-level node discovery contract consumed by the client.
type ServiceDiscovery interface {
	FinderService
	GetNodes() []*NodeInfo
	GetNode(id string) (*NodeInfo, bool)
	Watch(callback func([]*NodeInfo)) error
}

// Manager aggregates multiple discovery backends behind one interface.
type Manager interface {
	ServiceDiscovery
	AddFinder(name string, finder ServiceDiscovery, cacheTime, negCacheTime time.Duration) error
	RemoveFinder(name string) error
	ChildErrors() map[string]error
}

type cache struct {
	entries map[string]CacheEntry
	mu      sync.Mutex
}

func newCache() *cache {
	return &cache{entries: make(map[string]CacheEntry)}
}

func (c *cache) Set(id string, entry CacheEntry) {
	c.mu.Lock()
	c.entries[id] = entry
	c.mu.Unlock()
}

func (c *cache) Get(id string) (CacheEntry, bool) {
	c.mu.Lock()
	entry, ok := c.entries[id]
	c.mu.Unlock()
	return entry, ok
}

func (c *cache) Snapshot() map[string]CacheEntry {
	c.mu.Lock()
	out := make(map[string]CacheEntry, len(c.entries))
	for id, entry := range c.entries {
		out[id] = entry
	}
	c.mu.Unlock()
	return out
}

type managedFinder struct {
	ServiceDiscovery
	cacheTime    time.Duration
	negCacheTime time.Duration
	cache        *cache
}

type manager struct {
	mu       sync.RWMutex
	finders  map[string]managedFinder
	watchers []func([]*NodeInfo)
	ctx      context.Context
	running  bool
}

func NewManager() Manager {
	return &manager{
		finders:  make(map[string]managedFinder),
		watchers: make([]func([]*NodeInfo), 0),
	}
}

func (m *manager) AddFinder(name string, finder ServiceDiscovery, cacheTime, negCacheTime time.Duration) error {
	if finder == nil {
		return fmt.Errorf("finder cannot be nil")
	}
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("finder name cannot be empty")
	}

	m.mu.Lock()
	if _, exists := m.finders[name]; exists {
		m.mu.Unlock()
		return fmt.Errorf("finder already registered: %s", name)
	}

	entry := managedFinder{
		ServiceDiscovery: finder,
		cacheTime:        cacheTime,
		negCacheTime:     negCacheTime,
		cache:            newCache(),
	}
	m.finders[name] = entry
	running := m.running
	ctx := m.ctx
	m.mu.Unlock()

	if err := finder.Watch(func([]*NodeInfo) {
		m.notifyWatchers()
	}); err != nil {
		m.mu.Lock()
		delete(m.finders, name)
		m.mu.Unlock()
		return err
	}

	if running {
		if err := finder.Start(ctx); err != nil {
			m.mu.Lock()
			delete(m.finders, name)
			m.mu.Unlock()
			return err
		}
		m.notifyWatchers()
	}

	return nil
}

func (m *manager) RemoveFinder(name string) error {
	m.mu.Lock()
	entry, exists := m.finders[name]
	if exists {
		delete(m.finders, name)
	}
	m.mu.Unlock()

	if !exists {
		return nil
	}

	if err := entry.Stop(); err != nil {
		return err
	}
	m.notifyWatchers()
	return nil
}

func (m *manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("discovery manager is already running")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	m.ctx = ctx

	for name, finder := range m.finders {
		if err := finder.Start(ctx); err != nil {
			return fmt.Errorf("failed to start finder %s: %w", name, err)
		}
	}

	m.running = true
	go m.notifyWatchers()
	return nil
}

func (m *manager) Stop() error {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return nil
	}

	entries := make([]managedFinder, 0, len(m.finders))
	for _, finder := range m.finders {
		entries = append(entries, finder)
	}

	m.running = false
	m.ctx = nil
	m.mu.Unlock()

	var stopErr error
	for _, finder := range entries {
		if err := finder.Stop(); err != nil && stopErr == nil {
			stopErr = err
		}
	}

	m.notifyWatchers()
	return stopErr
}

func (m *manager) Lookup(ctx context.Context, nodeID string) ([]string, error) {
	m.mu.RLock()
	finders := make(map[string]managedFinder, len(m.finders))
	for name, finder := range m.finders {
		finders[name] = finder
	}
	m.mu.RUnlock()

	var addresses []string
	for _, finder := range finders {
		if cacheEntry, ok := finder.cache.Get(nodeID); ok {
			if cacheEntry.Found && finder.cacheTime > 0 && time.Since(cacheEntry.When) < finder.cacheTime {
				addresses = append(addresses, cacheEntry.Addresses...)
				continue
			}
			if !cacheEntry.Found && finder.negCacheTime > 0 && time.Since(cacheEntry.When) < finder.negCacheTime {
				continue
			}
		}

		addrs, err := finder.Lookup(ctx, nodeID)
		if err != nil {
			finder.cache.Set(nodeID, CacheEntry{When: time.Now(), Found: false})
			continue
		}

		entry := CacheEntry{
			Addresses: UniqueTrimmedStrings(addrs),
			When:      time.Now(),
			Found:     len(addrs) > 0,
		}
		finder.cache.Set(nodeID, entry)
		addresses = append(addresses, entry.Addresses...)
	}

	addresses = UniqueTrimmedStrings(addresses)
	sort.Strings(addresses)
	return addresses, nil
}

func (m *manager) Error() error {
	childErrors := m.ChildErrors()
	for _, err := range childErrors {
		return err
	}
	return nil
}

func (m *manager) String() string {
	return "discovery-manager"
}

func (m *manager) Cache() map[string]CacheEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make(map[string]CacheEntry)
	for _, finder := range m.finders {
		for nodeID, entry := range finder.cache.Snapshot() {
			out[nodeID] = entry
		}
	}
	return out
}

func (m *manager) ChildErrors() map[string]error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make(map[string]error)
	for name, finder := range m.finders {
		if err := finder.Error(); err != nil {
			out[name] = err
		}
	}
	return out
}

func (m *manager) GetNodes() []*NodeInfo {
	m.mu.RLock()
	finders := make([]managedFinder, 0, len(m.finders))
	for _, finder := range m.finders {
		finders = append(finders, finder)
	}
	m.mu.RUnlock()

	merged := make(map[string]*NodeInfo)
	for _, finder := range finders {
		for _, node := range finder.GetNodes() {
			if node == nil {
				continue
			}
			existing, ok := merged[node.ID]
			if !ok {
				merged[node.ID] = CloneNode(node)
				continue
			}
			merged[node.ID] = mergeNodeInfo(existing, node)
		}
	}

	nodes := make([]*NodeInfo, 0, len(merged))
	for _, node := range merged {
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	return nodes
}

func mergeNodeInfo(existing, incoming *NodeInfo) *NodeInfo {
	if existing == nil {
		return CloneNode(incoming)
	}
	if incoming == nil {
		return CloneNode(existing)
	}

	primary := existing
	secondary := incoming
	if incoming.LastSeen.After(existing.LastSeen) {
		primary = incoming
		secondary = existing
	}

	merged := CloneNode(primary)

	if merged.Name == "" && secondary.Name != "" {
		merged.Name = secondary.Name
	}
	if merged.Address == "" && secondary.Address != "" {
		merged.Address = secondary.Address
	}
	if merged.Version == "" && secondary.Version != "" {
		merged.Version = secondary.Version
	}
	if merged.Runtime == "" && secondary.Runtime != "" {
		merged.Runtime = secondary.Runtime
	}
	if merged.Metadata == nil && secondary.Metadata != nil {
		merged.Metadata = make(map[string]interface{}, len(secondary.Metadata))
		for k, v := range secondary.Metadata {
			merged.Metadata[k] = v
		}
	}
	if len(merged.Models) == 0 && len(secondary.Models) > 0 {
		merged.Models = CloneNode(secondary).Models
	}
	if len(merged.Tasks) == 0 && len(secondary.Tasks) > 0 {
		merged.Tasks = CloneIOTasks(secondary.Tasks)
	}
	if len(merged.Capabilities) == 0 && len(secondary.Capabilities) > 0 {
		merged.Capabilities = CloneCapabilities(secondary.Capabilities)
	}
	if merged.Load == nil && secondary.Load != nil {
		loadCopy := *secondary.Load
		merged.Load = &loadCopy
	}
	if merged.Stats == nil && secondary.Stats != nil {
		statsCopy := *secondary.Stats
		merged.Stats = &statsCopy
	}
	if merged.Status != NodeStatusActive && secondary.Status == NodeStatusActive {
		merged.Status = NodeStatusActive
	}
	if merged.Weight == 0 && secondary.Weight != 0 {
		merged.Weight = secondary.Weight
	}

	return merged
}

func (m *manager) GetNode(id string) (*NodeInfo, bool) {
	nodes := m.GetNodes()
	for _, node := range nodes {
		if node.ID == id {
			return node, true
		}
	}
	return nil, false
}

func (m *manager) Watch(callback func([]*NodeInfo)) error {
	if callback == nil {
		return fmt.Errorf("callback cannot be nil")
	}

	m.mu.Lock()
	m.watchers = append(m.watchers, callback)
	m.mu.Unlock()

	go callback(m.GetNodes())
	return nil
}

func (m *manager) notifyWatchers() {
	m.mu.RLock()
	watchers := make([]func([]*NodeInfo), len(m.watchers))
	copy(watchers, m.watchers)
	m.mu.RUnlock()

	if len(watchers) == 0 {
		return
	}

	nodes := m.GetNodes()
	for _, watcher := range watchers {
		go watcher(nodes)
	}
}

func UniqueTrimmedStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
