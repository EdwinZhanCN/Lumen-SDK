package client

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/edwinzhancn/lumen-sdk/pkg/discovery"
	pb "github.com/edwinzhancn/lumen-sdk/proto"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// --- shouldAffectNodeHealth tests (pure functions, unchanged) ---

func TestShouldAffectNodeHealth(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"context canceled", context.Canceled, false},
		{"context deadline", context.DeadlineExceeded, false},
		{"grpc canceled", status.Error(codes.Canceled, "canceled"), false},
		{"grpc deadline", status.Error(codes.DeadlineExceeded, "deadline"), false},
		{"grpc invalid argument", status.Error(codes.InvalidArgument, "bad input"), false},
		{"grpc failed precondition", status.Error(codes.FailedPrecondition, "not ready"), false},
		{"grpc resource exhausted", status.Error(codes.ResourceExhausted, "oom"), false},
		{"grpc internal", status.Error(codes.Internal, "model failed"), false},
		{"grpc not found", status.Error(codes.NotFound, "missing"), false},
		{"grpc already exists", status.Error(codes.AlreadyExists, "duplicate"), false},
		{"grpc permission denied", status.Error(codes.PermissionDenied, "denied"), false},
		{"grpc unauthenticated", status.Error(codes.Unauthenticated, "auth"), false},
		{"grpc out of range", status.Error(codes.OutOfRange, "range"), false},
		{"grpc unimplemented", status.Error(codes.Unimplemented, "method"), false},
		{"grpc unavailable", status.Error(codes.Unavailable, "offline"), true},
		{"transport eof", io.EOF, true},
		{"stream broken", errors.New("stream broken"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldAffectNodeHealth(context.Background(), tt.err); got != tt.want {
				t.Fatalf("shouldAffectNodeHealth() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShouldAffectNodeHealthCanceledContextWins(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if shouldAffectNodeHealth(ctx, status.Error(codes.Unavailable, "offline")) {
		t.Fatal("canceled context should not affect node health")
	}
	if shouldAffectNodeHealth(ctx, errors.New("transport eof")) {
		t.Fatal("canceled context should ignore non-status transport errors")
	}
}

// --- filterByTask tests (test picker filtering logic without needing real SubConns) ---

func TestFilterByTaskSelectsMatchingNodes(t *testing.T) {
	nodes := []*subConnState{
		{tasks: []string{"ocr", "embed"}, state: connectivity.Ready},
		{tasks: []string{"semantic"}, state: connectivity.Ready},
		{tasks: []string{"ocr"}, state: connectivity.Ready},
	}
	now := time.Now()

	result := filterByTask(nodes, "ocr", false, now)
	if len(result) != 2 {
		t.Fatalf("expected 2 matches for 'ocr', got %d", len(result))
	}

	result = filterByTask(nodes, "semantic", false, now)
	if len(result) != 1 {
		t.Fatalf("expected 1 match for 'semantic', got %d", len(result))
	}

	result = filterByTask(nodes, "nonexistent", false, now)
	if len(result) != 0 {
		t.Fatalf("expected 0 matches for 'nonexistent', got %d", len(result))
	}
}

func TestFilterByTaskEmptyTaskReturnsAll(t *testing.T) {
	nodes := []*subConnState{
		{tasks: []string{"ocr"}, state: connectivity.Ready},
		{tasks: []string{"semantic"}, state: connectivity.Ready},
	}
	result := filterByTask(nodes, "", false, time.Now())
	if len(result) != 2 {
		t.Fatalf("empty task should match all, got %d", len(result))
	}
}

func TestFilterByTaskSkipsCoolingNodes(t *testing.T) {
	now := time.Now()
	nodes := []*subConnState{
		{tasks: []string{"ocr"}, state: connectivity.Ready, cooldownUntil: now.Add(time.Hour)},
		{tasks: []string{"ocr"}, state: connectivity.Ready},
	}
	result := filterByTask(nodes, "ocr", false, now)
	if len(result) != 1 {
		t.Fatalf("expected 1 (skip cooling node), got %d", len(result))
	}
}

func TestFilterByTaskExpiredCooldownProbes(t *testing.T) {
	now := time.Now()
	nodes := []*subConnState{
		{tasks: []string{"ocr"}, state: connectivity.TransientFailure, cooldownUntil: now.Add(-time.Millisecond)},
		{tasks: []string{"ocr"}, state: connectivity.TransientFailure, cooldownUntil: now.Add(time.Hour)},
	}
	result := filterByTask(nodes, "ocr", true, now)
	if len(result) != 1 {
		t.Fatalf("expected 1 probe with expired cooldown, got %d", len(result))
	}
}

func TestAnySupportsTask(t *testing.T) {
	nodes := []*subConnState{
		{tasks: []string{"ocr", "embed"}},
		{tasks: []string{"semantic"}},
	}
	if !anySupportsTask(nodes, "ocr") {
		t.Fatal("should support ocr")
	}
	if !anySupportsTask(nodes, "semantic") {
		t.Fatal("should support semantic")
	}
	if anySupportsTask(nodes, "nonexistent") {
		t.Fatal("should not support nonexistent")
	}
}

// --- Cooldown logic tests ---

func TestStartCooldownExponentialBackoff(t *testing.T) {
	lb := &lumenBalancer{
		options: balancerOptions{
			rediscoveryBackoffMin: time.Second,
			rediscoveryBackoffMax: time.Minute,
		},
	}

	scs := &subConnState{}
	now := time.Now()

	lb.startCooldownLocked(scs, now)
	if scs.cooldown != time.Second {
		t.Fatalf("first cooldown = %v, want 1s", scs.cooldown)
	}

	lb.startCooldownLocked(scs, now)
	if scs.cooldown != 2*time.Second {
		t.Fatalf("second cooldown = %v, want 2s", scs.cooldown)
	}

	lb.startCooldownLocked(scs, now)
	if scs.cooldown != 4*time.Second {
		t.Fatalf("third cooldown = %v, want 4s", scs.cooldown)
	}
}

func TestStartCooldownCapsAtMax(t *testing.T) {
	lb := &lumenBalancer{
		options: balancerOptions{
			rediscoveryBackoffMin: 30 * time.Second,
			rediscoveryBackoffMax: time.Minute,
		},
	}
	scs := &subConnState{cooldown: 30 * time.Second}
	now := time.Now()

	lb.startCooldownLocked(scs, now)
	if scs.cooldown != time.Minute {
		t.Fatalf("cooldown = %v, want 1m (capped)", scs.cooldown)
	}

	lb.startCooldownLocked(scs, now)
	if scs.cooldown != time.Minute {
		t.Fatalf("cooldown = %v, should stay at max 1m", scs.cooldown)
	}
}

// --- Task context tests ---

func TestTaskContext(t *testing.T) {
	ctx := context.Background()
	if got := TaskFromContext(ctx); got != "" {
		t.Fatalf("empty context task = %q, want empty", got)
	}

	ctx = WithTask(ctx, "ocr")
	if got := TaskFromContext(ctx); got != "ocr" {
		t.Fatalf("task = %q, want ocr", got)
	}
}

// --- Node registry tests ---

func TestNodeRegistryStats(t *testing.T) {
	reg := &nodeRegistry{
		nodes: map[string]*registeredNode{
			"node-1": {state: connectivity.Ready},
			"node-2": {state: connectivity.Connecting},
			"node-3": {state: connectivity.Ready},
		},
	}
	total, healthy := reg.stats()
	if total != 3 || healthy != 2 {
		t.Fatalf("stats = (%d, %d), want (3, 2)", total, healthy)
	}
}

func TestNodeRegistryNodeInfos(t *testing.T) {
	reg := &nodeRegistry{
		nodes: map[string]*registeredNode{
			"local-node-1": {
				identity: discovery.NewNodeIdentity("local", "node-1"),
				addr:     "192.168.1.10:5866",
				state:    connectivity.Ready,
				tasks:    []string{"ocr", "embed"},
				txt:      map[string]string{"v": "1.2.3", "runtime": "onnxrt"},
			},
		},
	}
	infos := reg.nodeInfos()
	if len(infos) != 1 {
		t.Fatalf("nodeInfos len = %d, want 1", len(infos))
	}
	if infos[0].Version != "1.2.3" || infos[0].Runtime != "onnxrt" {
		t.Fatalf("unexpected info: %+v", infos[0])
	}
	if infos[0].Availability != discovery.NodeAvailabilityReady {
		t.Fatalf("availability = %s, want ready", infos[0].Availability)
	}
}

func TestNodeRegistryEmptyStats(t *testing.T) {
	reg := &nodeRegistry{nodes: make(map[string]*registeredNode)}
	total, healthy := reg.stats()
	if total != 0 || healthy != 0 {
		t.Fatalf("empty stats = (%d, %d), want (0, 0)", total, healthy)
	}
}

// --- availabilityFromRegistered tests ---

func TestAvailabilityFromRegistered(t *testing.T) {
	tests := []struct {
		name   string
		node   registeredNode
		expect discovery.NodeAvailability
	}{
		{"ready", registeredNode{state: connectivity.Ready}, discovery.NodeAvailabilityReady},
		{"connecting", registeredNode{state: connectivity.Connecting}, discovery.NodeAvailabilityConnecting},
		{"idle", registeredNode{state: connectivity.Idle}, discovery.NodeAvailabilityResolving},
		{"transient failure below threshold", registeredNode{state: connectivity.TransientFailure, hardFailures: 1}, discovery.NodeAvailabilityRediscovering},
		{"transient failure at threshold", registeredNode{state: connectivity.TransientFailure, hardFailures: hardFailureThreshold}, discovery.NodeAvailabilityUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := availabilityFromRegistered(&tt.node)
			if got != tt.expect {
				t.Fatalf("availability = %s, want %s", got, tt.expect)
			}
		})
	}
}

// --- Helper function tests ---

func TestMergeTasks(t *testing.T) {
	result := mergeTasks([]string{"ocr", "embed"}, []string{"embed", "semantic"})
	if len(result) != 3 {
		t.Fatalf("merged len = %d, want 3", len(result))
	}
	expected := map[string]bool{"ocr": true, "embed": true, "semantic": true}
	for _, task := range result {
		if !expected[task] {
			t.Fatalf("unexpected task: %s", task)
		}
	}
}

func TestMergeTasksTrimsWhitespace(t *testing.T) {
	result := mergeTasks([]string{" ocr ", ""}, []string{" embed "})
	if len(result) != 2 {
		t.Fatalf("merged len = %d, want 2", len(result))
	}
}

func TestNodeSupportsTaskSlice(t *testing.T) {
	tasks := []string{"ocr", "embed", "semantic"}
	if !nodeSupportsTaskSlice(tasks, "ocr") {
		t.Fatal("should support ocr")
	}
	if nodeSupportsTaskSlice(tasks, "missing") {
		t.Fatal("should not support missing")
	}
	if nodeSupportsTaskSlice(nil, "ocr") {
		t.Fatal("nil tasks should not support anything")
	}
}

// --- Integration tests with real gRPC server ---

func TestPoolConnectAndDiscoverNode(t *testing.T) {
	addr := startCapabilityServer(t, "semantic")

	host, port, err := splitEndpoint(addr)
	if err != nil {
		t.Fatal(err)
	}

	resolver := &fakeNodeResolver{
		events: []discovery.NodeEvent{
			{
				Type: discovery.NodeDiscovered,
				Resolved: discovery.ResolvedNode{
					Identity:  discovery.NewNodeIdentity("local", "node-1"),
					Addresses: []string{host},
					Port:      port,
					Txt:       map[string]string{"tasks": "semantic"},
				},
			},
		},
	}

	pool := NewPoolWithOptions(zap.NewNop(), PoolOptions{
		ConnectTimeout:        2 * time.Second,
		RediscoveryBackoffMin: time.Second,
		RediscoveryBackoffMax: 5 * time.Second,
	})

	if err := pool.Connect(resolver); err != nil {
		t.Fatalf("Connect error: %v", err)
	}
	defer pool.Close()

	waitUntil(t, func() bool {
		s := pool.Stats()
		return s.TotalConnections > 0 && s.HealthyConnections > 0
	})

	infos := pool.NodeInfos()
	if len(infos) == 0 {
		t.Fatal("expected at least one node info")
	}

	waitUntil(t, func() bool {
		for _, info := range pool.NodeInfos() {
			for _, task := range info.Tasks {
				if task.Name == "semantic" {
					return true
				}
			}
		}
		return false
	})
}

func TestPoolMultipleNodes(t *testing.T) {
	addr1 := startCapabilityServer(t, "ocr")
	addr2 := startCapabilityServer(t, "semantic")

	host1, port1, _ := splitEndpoint(addr1)
	host2, port2, _ := splitEndpoint(addr2)

	resolver := &fakeNodeResolver{
		events: []discovery.NodeEvent{
			{
				Type: discovery.NodeDiscovered,
				Resolved: discovery.ResolvedNode{
					Identity:  discovery.NewNodeIdentity("local", "node-1"),
					Addresses: []string{host1},
					Port:      port1,
					Txt:       map[string]string{"tasks": "ocr"},
				},
			},
			{
				Type: discovery.NodeDiscovered,
				Resolved: discovery.ResolvedNode{
					Identity:  discovery.NewNodeIdentity("local", "node-2"),
					Addresses: []string{host2},
					Port:      port2,
					Txt:       map[string]string{"tasks": "semantic"},
				},
			},
		},
	}

	pool := NewPoolWithOptions(zap.NewNop(), PoolOptions{
		ConnectTimeout:        2 * time.Second,
		RediscoveryBackoffMin: time.Second,
		RediscoveryBackoffMax: 5 * time.Second,
	})

	if err := pool.Connect(resolver); err != nil {
		t.Fatalf("Connect error: %v", err)
	}
	defer pool.Close()

	waitUntil(t, func() bool {
		s := pool.Stats()
		return s.TotalConnections >= 2 && s.HealthyConnections >= 2
	})

	waitUntil(t, func() bool {
		infos := pool.NodeInfos()
		if len(infos) < 2 {
			return false
		}
		foundOCR, foundSemantic := false, false
		for _, info := range infos {
			for _, task := range info.Tasks {
				if task.Name == "ocr" {
					foundOCR = true
				}
				if task.Name == "semantic" {
					foundSemantic = true
				}
			}
		}
		return foundOCR && foundSemantic
	})
}

func TestPoolWatcherNotification(t *testing.T) {
	addr := startCapabilityServer(t, "ocr")
	host, port, _ := splitEndpoint(addr)

	resolver := &fakeNodeResolver{
		events: []discovery.NodeEvent{
			{
				Type: discovery.NodeDiscovered,
				Resolved: discovery.ResolvedNode{
					Identity:  discovery.NewNodeIdentity("local", "node-1"),
					Addresses: []string{host},
					Port:      port,
					Txt:       map[string]string{"tasks": "ocr"},
				},
			},
		},
	}

	pool := NewPoolWithOptions(zap.NewNop(), PoolOptions{
		ConnectTimeout:        2 * time.Second,
		RediscoveryBackoffMin: time.Second,
		RediscoveryBackoffMax: 5 * time.Second,
	})

	notified := make(chan struct{}, 10)
	pool.OnNodesChanged(func(nodes []*discovery.NodeInfo) {
		select {
		case notified <- struct{}{}:
		default:
		}
	})

	if err := pool.Connect(resolver); err != nil {
		t.Fatalf("Connect error: %v", err)
	}
	defer pool.Close()

	select {
	case <-notified:
	case <-time.After(5 * time.Second):
		t.Fatal("watcher not notified within timeout")
	}
}

func TestPoolCloseIdempotent(t *testing.T) {
	pool := NewPoolWithOptions(zap.NewNop(), PoolOptions{})
	resolver := &fakeNodeResolver{}

	if err := pool.Connect(resolver); err != nil {
		t.Fatal(err)
	}
	if err := pool.Close(); err != nil {
		t.Fatal(err)
	}
	if err := pool.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestPoolStatsBeforeConnect(t *testing.T) {
	pool := NewPoolWithOptions(zap.NewNop(), PoolOptions{})
	s := pool.Stats()
	if s.TotalConnections != 0 || s.HealthyConnections != 0 {
		t.Fatalf("stats before connect = %+v, want zeros", s)
	}
	infos := pool.NodeInfos()
	if len(infos) != 0 {
		t.Fatalf("nodeInfos before connect = %d, want 0", len(infos))
	}
}

// --- Helpers ---

func waitUntil(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition was not met before timeout")
}

func startCapabilityServer(t *testing.T, tasks ...string) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := grpc.NewServer()
	pb.RegisterInferenceServer(server, &testInferenceServer{tasks: tasks})
	go func() {
		_ = server.Serve(lis)
	}()
	t.Cleanup(func() {
		server.Stop()
		_ = lis.Close()
	})
	return lis.Addr().String()
}

type testInferenceServer struct {
	pb.UnimplementedInferenceServer
	tasks []string
}

func (s *testInferenceServer) GetCapabilities(context.Context, *emptypb.Empty) (*pb.Capability, error) {
	return s.capability(), nil
}

func (s *testInferenceServer) StreamCapabilities(_ *emptypb.Empty, stream grpc.ServerStreamingServer[pb.Capability]) error {
	return stream.Send(s.capability())
}

func (s *testInferenceServer) Health(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (s *testInferenceServer) capability() *pb.Capability {
	cap := &pb.Capability{
		ServiceName: "test",
		Tasks:       make([]*pb.IOTask, 0, len(s.tasks)),
	}
	for _, task := range s.tasks {
		cap.Tasks = append(cap.Tasks, &pb.IOTask{Name: task})
	}
	return cap
}

// fakeNodeResolver emits a fixed list of events then blocks until context done.
type fakeNodeResolver struct {
	events []discovery.NodeEvent
}

func (r *fakeNodeResolver) Watch(ctx context.Context) (<-chan discovery.NodeEvent, error) {
	ch := make(chan discovery.NodeEvent, len(r.events))
	for _, ev := range r.events {
		ch <- ev
	}
	go func() {
		<-ctx.Done()
		close(ch)
	}()
	return ch, nil
}

// --- Fake gRPC clients for testing ---

type fakeInferenceClient struct {
	inferErr        error
	stream          grpc.BidiStreamingClient[pb.InferRequest, pb.InferResponse]
	caps            []*pb.Capability
	capabilityCalls int
}

func (f *fakeInferenceClient) Infer(context.Context, ...grpc.CallOption) (grpc.BidiStreamingClient[pb.InferRequest, pb.InferResponse], error) {
	if f.inferErr != nil {
		return nil, f.inferErr
	}
	if f.stream != nil {
		return f.stream, nil
	}
	return &fakeInferStream{}, nil
}

func (f *fakeInferenceClient) GetCapabilities(context.Context, *emptypb.Empty, ...grpc.CallOption) (*pb.Capability, error) {
	return nil, nil
}

func (f *fakeInferenceClient) StreamCapabilities(context.Context, *emptypb.Empty, ...grpc.CallOption) (grpc.ServerStreamingClient[pb.Capability], error) {
	f.capabilityCalls++
	return &fakeCapabilityStream{caps: append([]*pb.Capability(nil), f.caps...)}, nil
}

func (f *fakeInferenceClient) Health(context.Context, *emptypb.Empty, ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, nil
}

type fakeCapabilityStream struct {
	caps []*pb.Capability
}

func (f *fakeCapabilityStream) Recv() (*pb.Capability, error) {
	if len(f.caps) == 0 {
		return nil, io.EOF
	}
	cap := f.caps[0]
	f.caps = f.caps[1:]
	return cap, nil
}

func (f *fakeCapabilityStream) Header() (metadata.MD, error) { return nil, nil }
func (f *fakeCapabilityStream) Trailer() metadata.MD         { return nil }
func (f *fakeCapabilityStream) CloseSend() error              { return nil }
func (f *fakeCapabilityStream) Context() context.Context      { return context.Background() }
func (f *fakeCapabilityStream) SendMsg(any) error             { return nil }
func (f *fakeCapabilityStream) RecvMsg(any) error             { return nil }

type fakeInferStream struct {
	sendErr      error
	closeSendErr error
	recvErr      error
	responses    []*pb.InferResponse
}

func (f *fakeInferStream) Send(*pb.InferRequest) error {
	return f.sendErr
}

func (f *fakeInferStream) Recv() (*pb.InferResponse, error) {
	if f.recvErr != nil {
		return nil, f.recvErr
	}
	if len(f.responses) == 0 {
		return nil, io.EOF
	}
	resp := f.responses[0]
	f.responses = f.responses[1:]
	return resp, nil
}

func (f *fakeInferStream) Header() (metadata.MD, error) { return nil, nil }
func (f *fakeInferStream) Trailer() metadata.MD         { return nil }
func (f *fakeInferStream) CloseSend() error              { return f.closeSendErr }
func (f *fakeInferStream) Context() context.Context      { return context.Background() }
func (f *fakeInferStream) SendMsg(any) error             { return nil }
func (f *fakeInferStream) RecvMsg(any) error             { return nil }
