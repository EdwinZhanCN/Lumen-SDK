package discovery

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/edwinzhancn/lumen-sdk/pkg/config"
	"github.com/grandcat/zeroconf"

	"go.uber.org/zap"
)

// MDNSResolver discovers ML nodes via mDNS (zeroconf) and emits NodeEvent
// values on a channel. It implements the NodeResolver interface.
//
// It runs a continuous mDNS browse to detect new nodes and a periodic scan to
// remove stale nodes that have disappeared from the network.
type MDNSResolver struct {
	serviceType  string
	domain       string
	scanInterval time.Duration
	logger       *zap.Logger
}

// NewMDNSResolver creates an mDNS-based resolver.
func NewMDNSResolver(cfg *config.DiscoveryConfig, logger *zap.Logger) *MDNSResolver {
	serviceType := "_lumen._tcp"
	domain := "local"
	scanInterval := 30 * time.Second
	if cfg != nil {
		if cfg.ServiceType != "" {
			serviceType = cfg.ServiceType
		}
		if cfg.Domain != "" {
			domain = cfg.Domain
		}
		if cfg.ScanInterval > 0 {
			scanInterval = cfg.ScanInterval
		}
	}
	return &MDNSResolver{
		serviceType:  serviceType,
		domain:       domain,
		scanInterval: scanInterval,
		logger:       ensureLogger(logger),
	}
}

// Watch starts mDNS browsing and emits node events on the returned channel.
func (r *MDNSResolver) Watch(ctx context.Context) (<-chan NodeEvent, error) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return nil, fmt.Errorf("create mDNS resolver: %w", err)
	}

	entries := make(chan *zeroconf.ServiceEntry, 16)
	if err := resolver.Browse(ctx, r.serviceType, r.domain, entries); err != nil {
		return nil, fmt.Errorf("mDNS browse: %w", err)
	}

	ch := make(chan NodeEvent, 32)

	go func() {
		defer close(ch)

		known := make(map[string]time.Time) // nodeID → last seen

		ticker := time.NewTicker(r.scanInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return

			case entry, ok := <-entries:
				if !ok {
					return
				}
				if entry == nil {
					continue
				}

				nodeID := r.nodeID(entry)
				addr := r.serviceAddress(entry)
				if addr == "" {
					continue
				}

				lastSeen, exists := known[nodeID]
				known[nodeID] = time.Now()

				if !exists {
					tasks := extractTasks(entry)
					r.logger.Info("mDNS node discovered",
						zap.String("id", nodeID),
						zap.String("addr", addr),
						zap.Strings("tasks", tasks),
					)
					ch <- NodeEvent{
						Type:    NodeAdded,
						NodeID:  nodeID,
						Address: addr,
						Tasks:   tasks,
					}
				} else {
					_ = lastSeen
					// refresh last-seen timestamp only
				}

			case <-ticker.C:
				now := time.Now()
				staleThreshold := r.scanInterval * 2
				for nodeID, lastSeen := range known {
					if now.Sub(lastSeen) > staleThreshold {
						delete(known, nodeID)
						r.logger.Info("mDNS node removed (stale)",
							zap.String("id", nodeID),
							zap.Duration("since_last_seen", now.Sub(lastSeen)),
						)
						ch <- NodeEvent{
							Type:   NodeRemoved,
							NodeID: nodeID,
						}
					}
				}
			}
		}
	}()

	return ch, nil
}

func (r *MDNSResolver) serviceAddress(entry *zeroconf.ServiceEntry) string {
	if len(entry.AddrIPv4) == 0 && len(entry.AddrIPv6) == 0 {
		return ""
	}
	var host string
	if len(entry.AddrIPv4) > 0 {
		host = entry.AddrIPv4[0].String()
	} else {
		host = "[" + entry.AddrIPv6[0].String() + "]"
	}
	return fmt.Sprintf("%s:%d", host, entry.Port)
}

func (r *MDNSResolver) nodeID(entry *zeroconf.ServiceEntry) string {
	if entry.Instance != "" {
		return entry.Instance
	}
	return fmt.Sprintf("%s:%d", entry.HostName, entry.Port)
}

// extractTasks reads task names from mDNS TXT records.
// ML nodes advertise supported tasks as TXT key "tasks" with comma-separated values.
func extractTasks(entry *zeroconf.ServiceEntry) []string {
	if entry == nil || len(entry.Text) == 0 {
		return nil
	}
	for _, txt := range entry.Text {
		const prefix = "tasks="
		if strings.HasPrefix(txt, prefix) {
			raw := strings.TrimPrefix(txt, prefix)
			if raw == "" {
				return nil
			}
			parts := strings.Split(raw, ",")
			var tasks []string
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p != "" {
					tasks = append(tasks, p)
				}
			}
			return tasks
		}
	}
	return nil
}
