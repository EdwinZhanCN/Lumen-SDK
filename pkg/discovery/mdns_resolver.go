package discovery

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/edwinzhancn/lumen-sdk/pkg/config"
	"go.uber.org/zap"
)

const (
	defaultPollInterval = 30 * time.Second
	defaultQueryTimeout = 3 * time.Second
	missThreshold       = 2
	maxDiagnosticCount  = 10_000
)

// ResolutionError is a typed internal diagnostic. Public status snapshots use
// Code only; Err is retained for structured SDK logs and internal consumers.
type ResolutionError struct {
	Code string
	Err  error
}

func (e *ResolutionError) Error() string {
	if e == nil || e.Err == nil {
		return e.Code
	}
	return e.Code + ": " + e.Err.Error()
}

func (e *ResolutionError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// MDNSResolver continuously reconciles bounded DNS-SD scans into one
// source-owned snapshot. Only successful complete scans contribute absence
// evidence; failed and timed-out scans preserve the previous snapshot.
type MDNSResolver struct {
	serviceType  string
	domain       string
	deploymentID string
	pollInterval time.Duration
	queryTimeout time.Duration
	query        mdnsQuery
	logger       *zap.Logger

	statusMu sync.RWMutex
	status   ResolverStatus
	refresh  chan struct{}
}

func NewMDNSResolver(cfg *config.DiscoveryConfig, logger *zap.Logger) *MDNSResolver {
	return newMDNSResolverWithQuery(cfg, logger, nil)
}

func newMDNSResolverWithQuery(cfg *config.DiscoveryConfig, logger *zap.Logger, query mdnsQuery) *MDNSResolver {
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
	logger = ensureLogger(logger)
	if query == nil {
		query = newUDPMDNSQuery()
	}
	return &MDNSResolver{
		serviceType:  strings.Trim(strings.TrimSpace(serviceType), "."),
		domain:       strings.Trim(strings.TrimSpace(domain), "."),
		deploymentID: deploymentID,
		pollInterval: pollInterval,
		queryTimeout: queryTimeout,
		query:        query,
		logger:       logger,
		status: ResolverStatus{
			Source: "mdns",
			State:  BackendStarting,
		},
		refresh: make(chan struct{}, 1),
	}
}

func (r *MDNSResolver) SourceName() string { return "mdns" }

func (r *MDNSResolver) ResolverStatuses() []ResolverStatus {
	r.statusMu.RLock()
	defer r.statusMu.RUnlock()
	return []ResolverStatus{r.status}
}

// ResolveNow coalesces expedited requests. It never replaces scheduled scans
// as the discovery liveness mechanism.
func (r *MDNSResolver) ResolveNow() {
	select {
	case r.refresh <- struct{}{}:
	default:
	}
}

func (r *MDNSResolver) Watch(ctx context.Context) (<-chan NodeEvent, error) {
	out := make(chan NodeEvent, 32)
	go r.watch(ctx, out)
	return out, nil
}

type knownNode struct {
	resolved ResolvedNode
	misses   int
}

func (r *MDNSResolver) watch(ctx context.Context, out chan<- NodeEvent) {
	defer close(out)
	emitter := newSnapshotEmitter()
	publisherDone := make(chan struct{})
	go func() {
		defer close(publisherDone)
		emitter.run(ctx, out)
	}()

	r.logger.Info("mDNS discovery backend started",
		zap.String("source", r.SourceName()),
		zap.String("service", r.serviceFQDN()),
		zap.Duration("scan_interval", r.pollInterval),
	)
	r.scanLoop(ctx, emitter)
	<-publisherDone
}

func (r *MDNSResolver) scanLoop(ctx context.Context, emitter *snapshotEmitter) {
	known := make(map[string]*knownNode)
	for {
		if ctx.Err() != nil {
			return
		}
		r.runScan(ctx, known, emitter)
		if ctx.Err() != nil {
			return
		}

		next := time.Now().UTC().Add(r.pollInterval)
		r.updateStatus(func(status *ResolverStatus) { status.NextScanAt = next })
		timer := time.NewTimer(r.pollInterval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return
		case <-r.refresh:
			if !timer.Stop() {
				<-timer.C
			}
		case <-timer.C:
		}
	}
}

func (r *MDNSResolver) runScan(ctx context.Context, known map[string]*knownNode, emitter *snapshotEmitter) {
	started := time.Now().UTC()
	r.updateStatus(func(status *ResolverStatus) {
		status.LastScanStartedAt = started
		status.NextScanAt = time.Time{}
		if status.LastScanCompletedAt.IsZero() {
			status.State = BackendStarting
		}
	})

	// The query gets a bounded collection window and a separate hard deadline,
	// so normal empty collection is a successful snapshot while a backend that
	// fails to return is a typed timeout.
	hardTimeout := r.queryTimeout + time.Second
	scanCtx, cancel := context.WithTimeout(ctx, hardTimeout)
	result := r.query.Scan(scanCtx, mdnsQueryRequest{
		ServiceName: r.serviceFQDN(),
		Window:      r.queryTimeout,
	})
	cancel()
	if result.StartedAt.IsZero() {
		result.StartedAt = started
	}
	if result.CompletedAt.IsZero() {
		result.CompletedAt = time.Now().UTC()
	}
	if result.Outcome == "" {
		result.Outcome, result.ErrorCode = classifyQueryError(result.Err)
	}

	if !result.successful() {
		r.recordFailedScan(result)
		return
	}

	observed := make(map[string]ResolvedNode)
	resolveFailed := make(map[string]struct{})
	rejected := clampDiagnosticCount(result.Rejected)
	lastCode := ErrorCodeNone
	for _, entry := range result.Entries {
		resolved, admission, resolutionErr := r.resolvedNodeFromEntry(entry, result.CompletedAt)
		switch admission {
		case mdnsAccepted:
			key := resolved.Key()
			if previous, ok := observed[key]; ok {
				observed[key] = mergeMDNSObservations(previous, resolved)
			} else {
				observed[key] = resolved
			}
		case mdnsResolveFailed:
			lastCode = ErrorCodeResolveFailed
			rejected = clampDiagnosticCount(rejected + 1)
			if !resolved.Identity.IsZero() {
				resolveFailed[resolved.Key()] = struct{}{}
			}
			r.logger.Debug("mDNS service address resolution failed",
				zap.String("source", r.SourceName()),
				zap.String("error_code", ErrorCodeResolveFailed),
				zap.Error(resolutionErr),
			)
		case mdnsRejected:
			rejected = clampDiagnosticCount(rejected + 1)
		}
	}

	for key, node := range observed {
		if previous, ok := known[key]; ok {
			if !sameObservationIgnoringTime(previous.resolved, node) {
				r.logger.Info("mDNS node observation updated",
					zap.String("source", r.SourceName()),
					zap.String("id", key),
				)
			}
			previous.resolved = node
			previous.misses = 0
			continue
		}
		known[key] = &knownNode{resolved: node}
		r.logger.Info("mDNS node added",
			zap.String("source", r.SourceName()),
			zap.String("id", key),
		)
	}

	for key, node := range known {
		if _, seen := observed[key]; seen {
			continue
		}
		if _, unresolved := resolveFailed[key]; unresolved {
			continue
		}
		node.misses++
		if node.misses < missThreshold {
			continue
		}
		delete(known, key)
		r.logger.Info("mDNS node expired",
			zap.String("source", r.SourceName()),
			zap.String("id", key),
			zap.Int("successful_omissions", node.misses),
		)
	}

	desired := make(map[string]ResolvedNode, len(known))
	for key, node := range known {
		desired[key] = node.resolved
	}
	emitter.replace(desired)

	r.updateStatus(func(status *ResolverStatus) {
		status.State = BackendHealthy
		status.LastScanStartedAt = result.StartedAt
		status.LastScanCompletedAt = result.CompletedAt
		status.LastScanSucceededAt = result.CompletedAt
		status.ConsecutiveFailures = 0
		status.LastErrorCode = lastCode
		status.MatchedCount = clampDiagnosticCount(len(observed))
		status.RejectedCount = rejected
		status.LastOutcome = ScanOutcomeSuccess
	})
	r.logger.Debug("mDNS scan completed",
		zap.String("source", r.SourceName()),
		zap.String("outcome", string(ScanOutcomeSuccess)),
		zap.Int("matched", len(observed)),
		zap.Int("rejected", rejected),
		zap.Duration("duration", result.CompletedAt.Sub(result.StartedAt)),
	)
}

func (r *MDNSResolver) recordFailedScan(result mdnsQueryResult) {
	code := result.ErrorCode
	if code == "" {
		code = ErrorCodeQueryFailed
	}
	r.updateStatus(func(status *ResolverStatus) {
		status.State = BackendDegraded
		status.LastScanStartedAt = result.StartedAt
		status.LastScanCompletedAt = result.CompletedAt
		status.ConsecutiveFailures++
		status.LastErrorCode = code
		status.LastOutcome = result.Outcome
	})
	r.logger.Warn("mDNS scan failed; preserving previous observations",
		zap.String("source", r.SourceName()),
		zap.String("outcome", string(result.Outcome)),
		zap.String("error_code", code),
		zap.Error(result.Err),
	)
}

func (r *MDNSResolver) updateStatus(update func(*ResolverStatus)) {
	r.statusMu.Lock()
	defer r.statusMu.Unlock()
	update(&r.status)
}

func (r *MDNSResolver) serviceFQDN() string {
	return canonicalDNSName(r.serviceType + "." + r.domain)
}

type mdnsAdmission uint8

const (
	mdnsRejected mdnsAdmission = iota
	mdnsAccepted
	mdnsResolveFailed
)

func (r *MDNSResolver) resolvedNodeFromEntry(entry mdnsQueryEntry, observedAt time.Time) (ResolvedNode, mdnsAdmission, error) {
	expectedService := r.serviceFQDN()
	if !equalDNSName(entry.PTRName, expectedService) {
		return ResolvedNode{}, mdnsRejected, nil
	}
	instance, ok := extractInstanceNameStrict(entry.InstanceName, r.serviceType, r.domain)
	if !ok || !equalDNSName(entry.SRVName, entry.InstanceName) {
		return ResolvedNode{}, mdnsRejected, nil
	}
	identity := ParseNodeIdentity(instance, r.deploymentID)
	base := ResolvedNode{
		Identity:     identity,
		InstanceName: instance,
		HostName:     strings.TrimSuffix(strings.TrimSpace(entry.HostName), "."),
		Port:         entry.Port,
		Txt:          parseTXT(entry.TXT),
		Source:       r.SourceName(),
		Sources:      []string{r.SourceName()},
		LastObserved: observedAt,
	}.Normalized()
	if identity.IsZero() || entry.Port <= 0 || entry.Port > 65535 || base.HostName == "" {
		return base, mdnsRejected, nil
	}
	for _, address := range normalizeAddresses(entry.Addresses) {
		if _, err := netip.ParseAddr(address); err == nil {
			base.Addresses = append(base.Addresses, address)
		}
	}
	base = base.Normalized()
	if len(base.Addresses) == 0 {
		err := &ResolutionError{Code: ErrorCodeResolveFailed, Err: fmt.Errorf("matching service has no usable address")}
		return base, mdnsResolveFailed, err
	}
	return base, mdnsAccepted, nil
}

// extractInstanceNameStrict accepts only a canonical configured service/domain
// suffix. It preserves DNS label escaping and never falls back to the first
// plausible-looking label on a mismatch.
func extractInstanceNameStrict(fullName, serviceType, domain string) (string, bool) {
	fullName = canonicalDNSName(fullName)
	suffix := "." + canonicalDNSName(strings.Trim(serviceType, ".")+"."+strings.Trim(domain, "."))
	if fullName == "" || len(fullName) <= len(suffix) || !strings.EqualFold(fullName[len(fullName)-len(suffix):], suffix) {
		return "", false
	}
	instance := fullName[:len(fullName)-len(suffix)]
	if instance == "" || hasUnescapedDot(instance) {
		return "", false
	}
	return instance, true
}

func hasUnescapedDot(value string) bool {
	escaped := false
	for _, char := range value {
		if char == '\\' {
			escaped = !escaped
			continue
		}
		if char == '.' && !escaped {
			return true
		}
		escaped = false
	}
	return false
}

func equalDNSName(left, right string) bool {
	return strings.EqualFold(canonicalDNSName(left), canonicalDNSName(right))
}

func classifyQueryError(err error) (ScanOutcome, string) {
	switch {
	case err == nil:
		return ScanOutcomeSuccess, ErrorCodeNone
	case errors.Is(err, context.DeadlineExceeded):
		return ScanOutcomeTimedOut, ErrorCodeQueryTimedOut
	case errors.Is(err, context.Canceled):
		return ScanOutcomeCancelled, ErrorCodeCancelled
	default:
		return ScanOutcomeFailed, ErrorCodeQueryFailed
	}
}

func mergeMDNSObservations(left, right ResolvedNode) ResolvedNode {
	left = left.Normalized()
	right = right.Normalized()
	if left.Port != right.Port || !equalDNSName(left.HostName, right.HostName) {
		candidates := []ResolvedNode{left, right}
		sort.Slice(candidates, func(i, j int) bool { return candidates[i].Endpoint() < candidates[j].Endpoint() })
		return candidates[0]
	}
	left.Addresses = normalizeAddresses(append(left.Addresses, right.Addresses...))
	for key, value := range right.Txt {
		left.Txt[key] = value
	}
	if right.LastObserved.After(left.LastObserved) {
		left.LastObserved = right.LastObserved
	}
	return left.Normalized()
}

func sameObservationIgnoringTime(left, right ResolvedNode) bool {
	left.LastObserved = time.Time{}
	right.LastObserved = time.Time{}
	return reflect.DeepEqual(left.Normalized(), right.Normalized())
}

func clampDiagnosticCount(value int) int {
	if value < 0 {
		return 0
	}
	if value > maxDiagnosticCount {
		return maxDiagnosticCount
	}
	return value
}

func eventFromResolved(eventType NodeEventType, resolved ResolvedNode) NodeEvent {
	return NodeEvent{Type: eventType, Resolved: resolved.Normalized()}
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
