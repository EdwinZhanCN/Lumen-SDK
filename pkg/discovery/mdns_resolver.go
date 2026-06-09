package discovery

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/edwinzhancn/lumen-sdk/pkg/config"
	"github.com/hashicorp/mdns"

	"go.uber.org/zap"
)

const (
	defaultPollInterval = 30 * time.Second
	defaultQueryTimeout = 3 * time.Second
	missThreshold       = 2
)

// MDNSResolver discovers ML nodes via mDNS and emits NodeEvent values on a
// channel. It implements the NodeResolver interface.
//
// It runs a polling loop that periodically queries for mDNS services. Nodes not
// seen for consecutive polls are expired.
type MDNSResolver struct {
	serviceType  string
	domain       string
	deploymentID string
	pollInterval time.Duration
	queryTimeout time.Duration
	logger       *zap.Logger
}

// NewMDNSResolver creates an mDNS-based resolver.
func NewMDNSResolver(cfg *config.DiscoveryConfig, logger *zap.Logger) *MDNSResolver {
	serviceType := "_lumen._tcp"
	domain := "local"
	deploymentID := DefaultDeploymentID
	pollInterval := defaultPollInterval
	queryTimeout := defaultQueryTimeout
	if cfg != nil {
		if cfg.ServiceType != "" {
			serviceType = cfg.ServiceType
		}
		if cfg.Domain != "" {
			domain = cfg.Domain
		}
		if cfg.DeploymentID != "" {
			deploymentID = cfg.DeploymentID
		}
		if cfg.ScanInterval > 0 {
			pollInterval = cfg.ScanInterval
		}
		if cfg.ResolveTimeout > 0 {
			queryTimeout = cfg.ResolveTimeout
		}
	}
	return &MDNSResolver{
		serviceType:  serviceType,
		domain:       domain,
		deploymentID: deploymentID,
		pollInterval: pollInterval,
		queryTimeout: queryTimeout,
		logger:       ensureLogger(logger),
	}
}

// Watch starts mDNS polling and emits NodeEvent values on the returned channel.
func (r *MDNSResolver) Watch(ctx context.Context) (<-chan NodeEvent, error) {
	ch := make(chan NodeEvent, 32)
	go r.pollLoop(ctx, ch)
	return ch, nil
}

type knownNode struct {
	resolved ResolvedNode
	misses   int
}

func (r *MDNSResolver) pollLoop(ctx context.Context, ch chan<- NodeEvent) {
	defer close(ch)

	known := make(map[string]*knownNode)

	for {
		seen := r.runQuery(ctx, ch, known)
		if ctx.Err() != nil {
			return
		}

		for key, kn := range known {
			if seen[key] {
				kn.misses = 0
				continue
			}
			kn.misses++
			if kn.misses >= missThreshold {
				event := eventFromResolved(NodeExpired, kn.resolved)
				r.logger.Info("mDNS node expired",
					zap.String("id", key),
					zap.Int("missed_polls", kn.misses),
				)
				select {
				case ch <- event:
				case <-ctx.Done():
					return
				}
				delete(known, key)
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(r.pollInterval):
		}
	}
}

func (r *MDNSResolver) runQuery(ctx context.Context, ch chan<- NodeEvent, known map[string]*knownNode) map[string]bool {
	seen := make(map[string]bool)

	entries := make(chan *mdns.ServiceEntry, 16)
	params := &mdns.QueryParam{
		Service:     r.serviceType,
		Domain:      r.domain,
		Timeout:     r.queryTimeout,
		Entries:     entries,
		DisableIPv6: true,
	}

	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)
		for entry := range entries {
			if entry == nil {
				continue
			}
			resolved := r.resolvedNodeFromMDNS(entry)
			if resolved.Identity.IsZero() {
				continue
			}
			key := resolved.Key()
			seen[key] = true

			if kn, exists := known[key]; exists {
				kn.resolved = resolved
				kn.misses = 0
			} else {
				known[key] = &knownNode{resolved: resolved}
			}

			event := eventFromResolved(NodeDiscovered, resolved)
			r.logger.Info("mDNS node resolved",
				zap.String("id", key),
				zap.Strings("addresses", event.Addresses),
			)
			select {
			case ch <- event:
			case <-ctx.Done():
				return
			}
		}
	}()

	queryCtx, cancel := context.WithTimeout(ctx, r.queryTimeout+time.Second)
	defer cancel()
	if err := mdns.QueryContext(queryCtx, params); err != nil && ctx.Err() == nil {
		r.logger.Warn("mDNS query failed", zap.Error(err))
	}
	close(entries)
	<-doneCh

	return seen
}

func (r *MDNSResolver) resolvedNodeFromMDNS(entry *mdns.ServiceEntry) ResolvedNode {
	if entry == nil {
		return ResolvedNode{}
	}

	instance := extractInstanceName(entry.Name, r.serviceType, r.domain)
	if instance == "" {
		instance = fmt.Sprintf("%s:%d", entry.Host, entry.Port)
	}
	identity := ParseNodeIdentity(instance, r.deploymentID)

	var addresses []string
	if entry.AddrV4 != nil {
		addresses = append(addresses, entry.AddrV4.String())
	}
	if entry.AddrV6IPAddr != nil && entry.AddrV6IPAddr.IP != nil {
		addresses = append(addresses, entry.AddrV6IPAddr.IP.String())
	} else if entry.AddrV6 != nil {
		addresses = append(addresses, entry.AddrV6.String())
	}

	return ResolvedNode{
		Identity:     identity,
		InstanceName: instance,
		HostName:     strings.TrimSuffix(entry.Host, "."),
		Addresses:    addresses,
		Port:         entry.Port,
		Txt:          parseTXT(entry.InfoFields),
	}.Normalized()
}

// extractInstanceName extracts the instance name from a full DNS-SD name.
// E.g. "lab-node-1._lumen._tcp.local." → "lab-node-1"
func extractInstanceName(fullName, serviceType, domain string) string {
	fullName = strings.TrimSuffix(fullName, ".")
	suffix := "." + serviceType + "." + domain
	if strings.HasSuffix(fullName, suffix) {
		return strings.TrimSuffix(fullName, suffix)
	}
	if idx := strings.Index(fullName, "."); idx > 0 {
		return fullName[:idx]
	}
	return fullName
}

func eventFromResolved(eventType NodeEventType, resolved ResolvedNode) NodeEvent {
	resolved = resolved.Normalized()
	endpoints := resolved.CandidateEndpoints()
	address := ""
	if len(endpoints) > 0 {
		address = endpoints[0]
	}
	return NodeEvent{
		Type:      eventType,
		Identity:  resolved.Identity,
		Resolved:  resolved,
		NodeID:    resolved.Key(),
		Address:   address,
		Addresses: endpoints,
		Tasks:     resolved.HintTasks(),
		Txt:       resolved.Txt,
	}
}

func parseTXT(records []string) map[string]string {
	out := make(map[string]string, len(records))
	for _, record := range records {
		key, value, ok := splitTXT(record)
		if !ok {
			continue
		}
		out[key] = value
	}
	return out
}

func splitTXT(record string) (string, string, bool) {
	for i := 0; i < len(record); i++ {
		if record[i] == '=' {
			key := record[:i]
			if key == "" {
				return "", "", false
			}
			return key, record[i+1:], true
		}
	}
	if record == "" {
		return "", "", false
	}
	return record, "", true
}
