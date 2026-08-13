package discovery

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/edwinzhancn/lumen-sdk/pkg/config"
)

type scriptedMDNSQuery struct {
	mu      sync.Mutex
	results []mdnsQueryResult
	calls   chan int
	active  atomic.Int32
}

func (q *scriptedMDNSQuery) Scan(ctx context.Context, _ mdnsQueryRequest) mdnsQueryResult {
	q.active.Add(1)
	defer q.active.Add(-1)

	q.mu.Lock()
	hasResult := len(q.results) > 0
	var result mdnsQueryResult
	if hasResult {
		result = q.results[0]
		q.results = q.results[1:]
	}
	q.mu.Unlock()
	if q.calls != nil {
		select {
		case q.calls <- 1:
		default:
		}
	}
	if !hasResult {
		<-ctx.Done()
		return mdnsQueryResult{Outcome: ScanOutcomeCancelled, ErrorCode: ErrorCodeCancelled, Err: ctx.Err()}
	}
	if result.StartedAt.IsZero() {
		result.StartedAt = time.Now().UTC()
	}
	if result.CompletedAt.IsZero() {
		result.CompletedAt = result.StartedAt.Add(time.Millisecond)
	}
	return result
}

func successfulScan(entries ...mdnsQueryEntry) mdnsQueryResult {
	return mdnsQueryResult{Entries: entries, Outcome: ScanOutcomeSuccess}
}

func validMDNSEntry(node string, port int, address string) mdnsQueryEntry {
	instance := node + "._lumen._tcp.local."
	return mdnsQueryEntry{
		PTRName:      "_lumen._tcp.local.",
		InstanceName: instance,
		SRVName:      instance,
		HostName:     node + ".local.",
		Port:         port,
		Addresses:    []string{address},
		TXT:          []string{"v=1.2.3", "runtime=onnxrt", "tasks=ignored"},
	}
}

func testMDNSConfig() *config.DiscoveryConfig {
	return &config.DiscoveryConfig{
		ServiceType:    "_lumen._tcp",
		Domain:         "local",
		DeploymentID:   "lab",
		ScanInterval:   time.Hour,
		ResolveTimeout: 50 * time.Millisecond,
	}
}

func TestParseNodeIdentity(t *testing.T) {
	tests := []struct {
		name       string
		instance   string
		defaultDep string
		want       NodeIdentity
	}{
		{"deployment node", "lab-node-1", "lab", NodeIdentity{DeploymentID: "lab", NodeID: "node-1"}},
		{"legacy node", "node-1", "local", NodeIdentity{DeploymentID: "local", NodeID: "node-1"}},
		{"empty default", "node-1", "", NodeIdentity{DeploymentID: "local", NodeID: "node-1"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseNodeIdentity(tt.instance, tt.defaultDep)
			if got != tt.want {
				t.Fatalf("ParseNodeIdentity() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestMDNSAdmissionRequiresCorrelatedServiceChain(t *testing.T) {
	resolver := newMDNSResolverWithQuery(testMDNSConfig(), nil, &scriptedMDNSQuery{})
	now := time.Now().UTC()

	valid := validMDNSEntry("lab-node-1", 5866, "192.168.1.20")
	resolved, admission, err := resolver.resolvedNodeFromEntry(valid, now)
	if err != nil || admission != mdnsAccepted {
		t.Fatalf("valid entry admission = %v, err = %v", admission, err)
	}
	if resolved.Key() != "lab-node-1" || resolved.Endpoint() != "192.168.1.20:5866" {
		t.Fatalf("unexpected resolved node: %+v", resolved)
	}
	if resolved.Source != "mdns" || !resolved.LastObserved.Equal(now) {
		t.Fatalf("source/observation missing: %+v", resolved)
	}

	tests := []struct {
		name  string
		entry mdnsQueryEntry
	}{
		{name: "foreign ptr", entry: func() mdnsQueryEntry {
			entry := valid
			entry.PTRName = "_spotify-connect._tcp.local."
			return entry
		}()},
		{name: "foreign instance suffix", entry: func() mdnsQueryEntry {
			entry := valid
			entry.InstanceName = "lab-node-1._smb._tcp.local."
			entry.SRVName = entry.InstanceName
			return entry
		}()},
		{name: "uncorrelated srv", entry: func() mdnsQueryEntry {
			entry := valid
			entry.SRVName = "another._lumen._tcp.local."
			return entry
		}()},
		{name: "malformed uuid service", entry: mdnsQueryEntry{
			PTRName: "_lumen._tcp.local.", InstanceName: "e9c._lumen._tcp.local.",
			SRVName: "e9c._lumen._tcp.local.", Port: 0, Addresses: []string{"127.0.0.1"},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, got, _ := resolver.resolvedNodeFromEntry(tt.entry, now)
			if got != mdnsRejected {
				t.Fatalf("admission = %v, want rejected", got)
			}
		})
	}
}

func TestExtractInstanceNameStrict(t *testing.T) {
	tests := []struct {
		name string
		full string
		want string
		ok   bool
	}{
		{name: "canonical", full: "Lab-Node-1._LUMEN._TCP.LOCAL.", want: "Lab-Node-1", ok: true},
		{name: "no trailing dot", full: "lab-node-1._lumen._tcp.local", want: "lab-node-1", ok: true},
		{name: "escaped label", full: `lab\.node._lumen._tcp.local.`, want: `lab\.node`, ok: true},
		{name: "foreign", full: "node-1._spotify._tcp.local.", ok: false},
		{name: "bare instance", full: "node-1", ok: false},
		{name: "extra unescaped label", full: "bad.node._lumen._tcp.local.", ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := extractInstanceNameStrict(tt.full, "_lumen._tcp", "local")
			if got != tt.want || ok != tt.ok {
				t.Fatalf("extractInstanceNameStrict() = %q, %v; want %q, %v", got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestMDNSObservationAddressOrderingIsStable(t *testing.T) {
	left := ResolvedNode{
		Identity:  NewNodeIdentity("local", "ordered-addresses"),
		Addresses: []string{"10.0.0.2", "10.0.0.1"},
		Port:      5866,
	}
	right := ResolvedNode{
		Identity:  NewNodeIdentity("local", "ordered-addresses"),
		Addresses: []string{"10.0.0.1", "10.0.0.2"},
		Port:      5866,
	}
	if !sameObservationIgnoringTime(left, right) {
		t.Fatal("DNS packet ordering must not create a node update")
	}
}

func TestMDNSForeignTrafficNeverEntersSnapshot(t *testing.T) {
	valid := validMDNSEntry("lab-node-1", 5866, "192.168.1.20")
	spotify := valid
	spotify.PTRName = "_spotify-connect._tcp.local."
	smb := valid
	smb.PTRName = "_smb._tcp.local."
	uuid := valid
	uuid.InstanceName = "0dcaf._googlecast._tcp.local."
	uuid.SRVName = uuid.InstanceName
	malformed := valid
	malformed.Port = 0

	query := &scriptedMDNSQuery{results: []mdnsQueryResult{successfulScan(spotify, smb, uuid, malformed, valid)}}
	resolver := newMDNSResolverWithQuery(testMDNSConfig(), nil, query)
	ctx, cancel := context.WithCancel(context.Background())
	stream, err := resolver.Watch(ctx)
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}
	event := awaitMDNSEvent(t, stream, time.Second)
	if event.Type != NodeDiscovered || event.Resolved.Key() != "lab-node-1" {
		t.Fatalf("unexpected event: %+v", event)
	}
	status := resolver.ResolverStatuses()[0]
	if status.MatchedCount != 1 || status.RejectedCount != 4 || status.State != BackendHealthy {
		t.Fatalf("unexpected status: %+v", status)
	}
	cancel()
	awaitMDNSClose(t, stream)
}

func TestMDNSMatchingServiceWithoutAddressIsDiagnosticOnly(t *testing.T) {
	entry := validMDNSEntry("lab-node-1", 5866, "")
	query := &scriptedMDNSQuery{results: []mdnsQueryResult{successfulScan(entry)}}
	resolver := newMDNSResolverWithQuery(testMDNSConfig(), nil, query)
	ctx, cancel := context.WithCancel(context.Background())
	stream, _ := resolver.Watch(ctx)
	deadline := time.Now().Add(time.Second)
	for {
		status := resolver.ResolverStatuses()[0]
		if !status.LastScanCompletedAt.IsZero() {
			if status.State != BackendHealthy || status.LastErrorCode != ErrorCodeResolveFailed || status.MatchedCount != 0 {
				t.Fatalf("resolution status = %+v", status)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for resolution diagnostic")
		}
		time.Sleep(time.Millisecond)
	}
	select {
	case event := <-stream:
		t.Fatalf("unresolved matching service entered node stream: %+v", event)
	case <-time.After(25 * time.Millisecond):
	}
	cancel()
	awaitMDNSClose(t, stream)
}

func TestMDNSFailedScansDoNotContributeExpiryMisses(t *testing.T) {
	valid := validMDNSEntry("lab-node-1", 5866, "192.168.1.20")
	query := &scriptedMDNSQuery{
		calls: make(chan int, 8),
		results: []mdnsQueryResult{
			successfulScan(valid),
			{Outcome: ScanOutcomeFailed, ErrorCode: ErrorCodeQueryFailed, Err: errors.New("scripted failure")},
			{Outcome: ScanOutcomeTimedOut, ErrorCode: ErrorCodeQueryTimedOut, Err: context.DeadlineExceeded},
			successfulScan(),
			successfulScan(),
		},
	}
	resolver := newMDNSResolverWithQuery(testMDNSConfig(), nil, query)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream, _ := resolver.Watch(ctx)
	first := awaitMDNSEvent(t, stream, time.Second)
	if first.Type != NodeDiscovered {
		t.Fatalf("first event = %v, want discovered", first.Type)
	}

	for scan := 2; scan <= 4; scan++ {
		resolver.ResolveNow()
		awaitMDNSCall(t, query.calls)
		select {
		case event := <-stream:
			t.Fatalf("scan %d unexpectedly changed snapshot: %+v", scan, event)
		case <-time.After(25 * time.Millisecond):
		}
	}
	resolver.ResolveNow()
	awaitMDNSCall(t, query.calls)
	expired := awaitMDNSEvent(t, stream, time.Second)
	if expired.Type != NodeExpired || expired.Resolved.Key() != "lab-node-1" {
		t.Fatalf("expiry event = %+v", expired)
	}
}

func TestMDNSSlowConsumerCannotBlockNextScanOrCleanup(t *testing.T) {
	firstEntries := make([]mdnsQueryEntry, 0, 80)
	for index := 0; index < 80; index++ {
		firstEntries = append(firstEntries, validMDNSEntry(fmt.Sprintf("lab-node-%03d", index), 5866, fmt.Sprintf("10.0.0.%d", index+1)))
	}
	query := &scriptedMDNSQuery{
		calls: make(chan int, 4),
		results: []mdnsQueryResult{
			successfulScan(firstEntries...),
			successfulScan(validMDNSEntry("lab-late-node", 5866, "10.1.0.1")),
		},
	}
	resolver := newMDNSResolverWithQuery(testMDNSConfig(), nil, query)
	ctx, cancel := context.WithCancel(context.Background())
	stream, _ := resolver.Watch(ctx)
	awaitMDNSCall(t, query.calls)

	// Do not consume the 32-element public buffer. The publisher will block,
	// but the scanner must still complete the expedited second snapshot.
	resolver.ResolveNow()
	awaitMDNSCall(t, query.calls)
	if active := query.active.Load(); active != 0 {
		t.Fatalf("query cleanup incomplete, active scans = %d", active)
	}

	cancel()
	awaitMDNSClose(t, stream)
}

func TestMDNSWatchCancellationCleansUpScan(t *testing.T) {
	query := &scriptedMDNSQuery{calls: make(chan int, 1)}
	resolver := newMDNSResolverWithQuery(testMDNSConfig(), nil, query)
	ctx, cancel := context.WithCancel(context.Background())
	stream, _ := resolver.Watch(ctx)
	awaitMDNSCall(t, query.calls)
	cancel()
	awaitMDNSClose(t, stream)
	if active := query.active.Load(); active != 0 {
		t.Fatalf("scan goroutine leaked after cancellation: %d", active)
	}
}

func awaitMDNSEvent(t *testing.T, stream <-chan NodeEvent, timeout time.Duration) NodeEvent {
	t.Helper()
	select {
	case event, ok := <-stream:
		if !ok {
			t.Fatal("event stream closed")
		}
		return event
	case <-time.After(timeout):
		t.Fatal("timed out waiting for mDNS event")
		return NodeEvent{}
	}
}

func awaitMDNSCall(t *testing.T, calls <-chan int) {
	t.Helper()
	select {
	case <-calls:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for scripted scan")
	}
}

func awaitMDNSClose(t *testing.T, stream <-chan NodeEvent) {
	t.Helper()
	deadline := time.After(time.Second)
	for {
		select {
		case _, ok := <-stream:
			if !ok {
				return
			}
		case <-deadline:
			t.Fatal("mDNS event stream did not close")
		}
	}
}
