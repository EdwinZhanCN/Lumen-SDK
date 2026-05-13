package client

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/edwinzhancn/lumen-sdk/pkg/config"
	"github.com/edwinzhancn/lumen-sdk/pkg/discovery"
	"github.com/edwinzhancn/lumen-sdk/pkg/types"
	pb "github.com/edwinzhancn/lumen-sdk/proto"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestNewLumenClientWithNilLogger(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Discovery.Enabled = false
	cfg.LoadBalancer.CacheEnabled = false
	cfg.LoadBalancer.HealthCheck = false

	c, err := NewLumenClient(cfg, nil)
	if err != nil {
		t.Fatalf("NewLumenClient() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := c.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	cancel()
	if err := c.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}

func TestNodeInfoInvalidateTaskCache(t *testing.T) {
	node := &discovery.NodeInfo{
		Tasks: []*pb.IOTask{{Name: "old_task"}},
	}

	if !node.SupportsTask("old_task") {
		t.Fatal("expected old_task to be supported before update")
	}

	node.Tasks = []*pb.IOTask{{Name: "new_task"}}
	node.InvalidateTaskCache()

	if node.SupportsTask("old_task") {
		t.Fatal("expected old_task to be unsupported after cache invalidation")
	}
	if !node.SupportsTask("new_task") {
		t.Fatal("expected new_task to be supported after cache invalidation")
	}
}

func TestManualDiscoveryAddNodeDoesNotDeadlock(t *testing.T) {
	disco := discovery.NewManualDiscovery(nil)
	done := make(chan struct{})

	go func() {
		disco.AddNode(&discovery.NodeInfo{
			ID:     "node-1",
			Name:   "node-1",
			Status: discovery.NodeStatusActive,
		})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("AddNode appears to be deadlocked")
	}

	if err := disco.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}

func TestCreateLoadBalancerWithNilLogger(t *testing.T) {
	lb := CreateLoadBalancer(LoadBalancerType("unknown"), &config.LoadBalancerConfig{
		CacheEnabled: false,
		HealthCheck:  false,
	}, nil)

	if lb == nil {
		t.Fatal("expected load balancer instance")
	}

	stats := lb.GetStats()
	if stats.Strategy != "round_robin" {
		t.Fatalf("expected fallback strategy round_robin, got %s", stats.Strategy)
	}
}

func TestDiscoveryManagerAggregatesFinders(t *testing.T) {
	manager := discovery.NewManager()
	manualA := discovery.NewManualDiscovery(nil)
	manualB := discovery.NewManualDiscovery(nil)

	if err := manager.AddFinder("manual-a", manualA, time.Minute, time.Second); err != nil {
		t.Fatalf("AddFinder(manual-a) error = %v", err)
	}
	if err := manager.AddFinder("manual-b", manualB, time.Minute, time.Second); err != nil {
		t.Fatalf("AddFinder(manual-b) error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := manager.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer func() {
		if err := manager.Stop(); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
	}()

	manualA.AddNode(&discovery.NodeInfo{
		ID:       "node-a",
		Name:     "node-a",
		Address:  "127.0.0.1:50051",
		Status:   discovery.NodeStatusActive,
		LastSeen: time.Now(),
	})
	manualB.AddNode(&discovery.NodeInfo{
		ID:       "node-b",
		Name:     "node-b",
		Address:  "127.0.0.2:50052",
		Status:   discovery.NodeStatusActive,
		LastSeen: time.Now(),
	})

	deadline := time.Now().Add(500 * time.Millisecond)
	for {
		nodes := manager.GetNodes()
		if len(nodes) == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected 2 aggregated nodes, got %d", len(nodes))
		}
		time.Sleep(10 * time.Millisecond)
	}

	addrs, err := manager.Lookup(context.Background(), "node-a")
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	if len(addrs) != 1 || addrs[0] != "127.0.0.1:50051" {
		t.Fatalf("unexpected lookup result: %v", addrs)
	}
}

func TestDiscoveryManagerPreservesRichNodeDataFromOlderDuplicate(t *testing.T) {
	manager := discovery.NewManager()
	manualA := discovery.NewManualDiscovery(nil)
	manualB := discovery.NewManualDiscovery(nil)

	if err := manager.AddFinder("manual-a", manualA, time.Minute, time.Second); err != nil {
		t.Fatalf("AddFinder(manual-a) error = %v", err)
	}
	if err := manager.AddFinder("manual-b", manualB, time.Minute, time.Second); err != nil {
		t.Fatalf("AddFinder(manual-b) error = %v", err)
	}

	older := time.Now().Add(-time.Minute)
	newer := time.Now()

	manualA.AddNode(&discovery.NodeInfo{
		ID:       "shared-node",
		Name:     "shared-node",
		Address:  "127.0.0.1:50051",
		Status:   discovery.NodeStatusStarting,
		LastSeen: older,
		Tasks:    []*pb.IOTask{{Name: "embedding"}},
		Capabilities: []*pb.Capability{
			{
				ServiceName: "embedder",
				Tasks:       []*pb.IOTask{{Name: "embedding"}},
			},
		},
	})
	manualB.AddNode(&discovery.NodeInfo{
		ID:       "shared-node",
		Name:     "shared-node",
		Address:  "127.0.0.2:50052",
		Status:   discovery.NodeStatusActive,
		LastSeen: newer,
	})

	nodes := manager.GetNodes()
	if len(nodes) != 1 {
		t.Fatalf("expected 1 merged node, got %d", len(nodes))
	}

	node := nodes[0]
	if node.Address != "127.0.0.2:50052" {
		t.Fatalf("expected newer address to win, got %s", node.Address)
	}
	if node.Status != discovery.NodeStatusActive {
		t.Fatalf("expected active status, got %s", node.Status)
	}
	if len(node.Tasks) != 1 || node.Tasks[0].Name != "embedding" {
		t.Fatalf("expected older task metadata to be preserved, got %+v", node.Tasks)
	}
	if len(node.Capabilities) != 1 || node.Capabilities[0].ServiceName != "embedder" {
		t.Fatalf("expected older capabilities to be preserved, got %+v", node.Capabilities)
	}
}

func TestDiscoveryManagerWatchReceivesAggregateSnapshot(t *testing.T) {
	manager := discovery.NewManager()
	manual := discovery.NewManualDiscovery(nil)
	if err := manager.AddFinder("manual", manual, time.Minute, time.Second); err != nil {
		t.Fatalf("AddFinder() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := manager.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer func() {
		if err := manager.Stop(); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
	}()

	updates := make(chan int, 4)
	if err := manager.Watch(func(nodes []*discovery.NodeInfo) {
		updates <- len(nodes)
	}); err != nil {
		t.Fatalf("Watch() error = %v", err)
	}

	manual.AddNode(&discovery.NodeInfo{
		ID:       "node-1",
		Name:     "node-1",
		Address:  "127.0.0.1:50051",
		Status:   discovery.NodeStatusActive,
		LastSeen: time.Now(),
	})

	timeout := time.After(2 * time.Second)
	for {
		select {
		case count := <-updates:
			if count == 1 {
				return
			}
		case <-timeout:
			t.Fatal("did not receive aggregated watcher update")
		}
	}
}

func TestGetNodesReturnsSnapshot(t *testing.T) {
	disco := discovery.NewManualDiscovery(nil)
	node := &discovery.NodeInfo{
		ID:      "node-1",
		Name:    "node-1",
		Address: "127.0.0.1:50051",
		Status:  discovery.NodeStatusActive,
		Tasks:   []*pb.IOTask{{Name: "embedding"}},
	}
	disco.AddNode(node)

	c := &LumenClient{discovery: disco}

	nodes := c.GetNodes()
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}

	nodes[0].Status = discovery.NodeStatusError
	nodes[0].Tasks[0].Name = "mutated"

	origin, ok := disco.GetNode("node-1")
	if !ok {
		t.Fatal("expected node in discovery")
	}
	if origin.Status != discovery.NodeStatusActive {
		t.Fatalf("expected internal status to remain active, got %s", origin.Status)
	}
	if origin.Tasks[0].Name != "embedding" {
		t.Fatalf("expected internal task name unchanged, got %s", origin.Tasks[0].Name)
	}
}

func TestGetNodeReturnsSnapshot(t *testing.T) {
	disco := discovery.NewManualDiscovery(nil)
	node := &discovery.NodeInfo{
		ID:     "node-1",
		Status: discovery.NodeStatusActive,
		Tasks:  []*pb.IOTask{{Name: "embedding"}},
	}
	disco.AddNode(node)

	c := &LumenClient{discovery: disco}
	snapshot, ok := c.GetNode("node-1")
	if !ok {
		t.Fatal("expected node snapshot")
	}

	snapshot.Status = discovery.NodeStatusError
	snapshot.Tasks[0].Name = "mutated"

	origin, _ := disco.GetNode("node-1")
	if origin.Status != discovery.NodeStatusActive {
		t.Fatalf("expected internal status to remain active, got %s", origin.Status)
	}
	if origin.Tasks[0].Name != "embedding" {
		t.Fatalf("expected internal task name unchanged, got %s", origin.Tasks[0].Name)
	}
}

func TestGetCapabilitiesReturnsSnapshot(t *testing.T) {
	disco := discovery.NewManualDiscovery(nil)
	node := &discovery.NodeInfo{
		ID:     "node-1",
		Status: discovery.NodeStatusActive,
		Capabilities: []*pb.Capability{
			{
				ServiceName: "svc",
				ModelIds:    []string{"m1"},
			},
		},
	}
	disco.AddNode(node)

	c := &LumenClient{discovery: disco}
	caps, err := c.GetCapabilities(context.Background(), "node-1")
	if err != nil {
		t.Fatalf("GetCapabilities() error = %v", err)
	}
	if len(caps) != 1 {
		t.Fatalf("expected 1 capability, got %d", len(caps))
	}

	caps[0].ServiceName = "mutated"
	caps[0].ModelIds[0] = "mutated-model"

	origin, _ := disco.GetNode("node-1")
	if origin.Capabilities[0].ServiceName != "svc" {
		t.Fatalf("expected internal capability service unchanged, got %s", origin.Capabilities[0].ServiceName)
	}
	if origin.Capabilities[0].ModelIds[0] != "m1" {
		t.Fatalf("expected internal model id unchanged, got %s", origin.Capabilities[0].ModelIds[0])
	}
}

func TestInferWithRetryRespectsContextCancellation(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Discovery.Enabled = false
	cfg.LoadBalancer.CacheEnabled = false
	cfg.LoadBalancer.HealthCheck = false

	c, err := NewLumenClient(cfg, nil)
	if err != nil {
		t.Fatalf("NewLumenClient() error = %v", err)
	}

	retryCtx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, err = c.InferWithRetry(retryCtx, &pb.InferRequest{Task: "missing_task"},
		WithWaitForTask(true),
		WithRetryInterval(1*time.Second),
		WithMaxWaitTime(30*time.Second),
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
}

func TestInferWithRetryRejectsNilRequest(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Discovery.Enabled = false
	cfg.LoadBalancer.CacheEnabled = false
	cfg.LoadBalancer.HealthCheck = false

	c, err := NewLumenClient(cfg, nil)
	if err != nil {
		t.Fatalf("NewLumenClient() error = %v", err)
	}

	_, err = c.InferWithRetry(context.Background(), nil)
	if err == nil || err.Error() != "request cannot be nil" {
		t.Fatalf("expected nil request error, got %v", err)
	}
}

func TestInferAssemblesChunkedResponseForSingleRequest(t *testing.T) {
	server := &chunkedInferenceServer{
		responses: []*pb.InferResponse{
			clientResponseChunk(0, 2, 0, "he", false),
			clientResponseChunk(1, 2, 2, "llo", true),
		},
		readUntilEOF: true,
		received:     make(chan []*pb.InferRequest, 1),
	}
	address, stop := startChunkedInferenceServer(t, server)
	defer stop()

	c := newChunkedResponseTestClient(t, address, config.ChunkConfig{EnableAuto: false})
	resp, err := c.Infer(context.Background(), &pb.InferRequest{
		CorrelationId: "corr-1",
		Task:          "test_task",
		Payload:       []byte("input"),
	})
	if err != nil {
		t.Fatalf("Infer() error = %v", err)
	}
	if string(resp.Result) != "hello" {
		t.Fatalf("expected assembled result hello, got %q", string(resp.Result))
	}

	reqs := <-server.received
	if len(reqs) != 1 {
		t.Fatalf("expected one request, got %d", len(reqs))
	}
}

func TestInferAssemblesChunkedResponseAfterChunkedRequestUpload(t *testing.T) {
	server := &chunkedInferenceServer{
		responses: []*pb.InferResponse{
			clientResponseChunk(0, 2, 0, "ok", false),
			clientResponseChunk(1, 2, 2, "!", true),
		},
		readUntilEOF: true,
		received:     make(chan []*pb.InferRequest, 1),
	}
	address, stop := startChunkedInferenceServer(t, server)
	defer stop()

	c := newChunkedResponseTestClient(t, address, config.ChunkConfig{
		EnableAuto:    true,
		Threshold:     2,
		MaxChunkBytes: 2,
	})
	resp, err := c.Infer(context.Background(), &pb.InferRequest{
		CorrelationId: "corr-1",
		Task:          "test_task",
		Payload:       []byte("abcdef"),
	})
	if err != nil {
		t.Fatalf("Infer() error = %v", err)
	}
	if string(resp.Result) != "ok!" {
		t.Fatalf("expected assembled result ok!, got %q", string(resp.Result))
	}

	reqs := <-server.received
	if len(reqs) != 3 {
		t.Fatalf("expected three uploaded chunks, got %d", len(reqs))
	}
	if reqs[2].Seq != 2 || reqs[2].Total != 3 || reqs[2].Offset != 4 {
		t.Fatalf("unexpected third request chunk markers: seq=%d total=%d offset=%d", reqs[2].Seq, reqs[2].Total, reqs[2].Offset)
	}
}

func TestInferStreamYieldsResponseChunksUnchanged(t *testing.T) {
	server := &chunkedInferenceServer{
		responses: []*pb.InferResponse{
			clientResponseChunk(0, 2, 0, "he", false),
			clientResponseChunk(1, 2, 2, "llo", true),
		},
		readUntilEOF: false,
		received:     make(chan []*pb.InferRequest, 1),
	}
	address, stop := startChunkedInferenceServer(t, server)
	defer stop()

	c := newChunkedResponseTestClient(t, address, config.ChunkConfig{EnableAuto: false})
	ch, err := c.InferStream(context.Background(), &pb.InferRequest{
		CorrelationId: "corr-1",
		Task:          "test_task",
		Payload:       []byte("input"),
	})
	if err != nil {
		t.Fatalf("InferStream() error = %v", err)
	}

	var got []*pb.InferResponse
	for resp := range ch {
		got = append(got, resp)
	}
	if len(got) != 2 {
		t.Fatalf("expected two streamed responses, got %d", len(got))
	}
	if string(got[0].Result) != "he" || got[0].Total != 2 || got[0].IsFinal {
		t.Fatalf("first response was modified: %+v", got[0])
	}
	if string(got[1].Result) != "llo" || got[1].Seq != 1 || !got[1].IsFinal {
		t.Fatalf("second response was modified: %+v", got[1])
	}
}

type chunkedInferenceServer struct {
	pb.UnimplementedInferenceServer
	responses    []*pb.InferResponse
	readUntilEOF bool
	received     chan []*pb.InferRequest
}

func (s *chunkedInferenceServer) Infer(stream grpc.BidiStreamingServer[pb.InferRequest, pb.InferResponse]) error {
	var reqs []*pb.InferRequest
	if s.readUntilEOF {
		for {
			req, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
			reqs = append(reqs, req)
		}
	} else {
		req, err := stream.Recv()
		if err != nil {
			return err
		}
		reqs = append(reqs, req)
	}

	if s.received != nil {
		s.received <- reqs
	}

	for _, resp := range s.responses {
		if err := stream.Send(resp); err != nil {
			return err
		}
	}
	return nil
}

func (s *chunkedInferenceServer) GetCapabilities(context.Context, *emptypb.Empty) (*pb.Capability, error) {
	return &pb.Capability{
		Tasks: []*pb.IOTask{{Name: "test_task"}},
	}, nil
}

func (s *chunkedInferenceServer) Health(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func startChunkedInferenceServer(t *testing.T, server *chunkedInferenceServer) (string, func()) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterInferenceServer(grpcServer, server)
	go func() {
		_ = grpcServer.Serve(listener)
	}()

	return listener.Addr().String(), func() {
		grpcServer.Stop()
		_ = listener.Close()
	}
}

func newChunkedResponseTestClient(t *testing.T, address string, chunkCfg config.ChunkConfig) *LumenClient {
	t.Helper()

	cfg := config.DefaultConfig()
	cfg.Discovery.Enabled = false
	cfg.LoadBalancer.CacheEnabled = false
	cfg.LoadBalancer.HealthCheck = false
	cfg.Chunk = chunkCfg

	lb := NewSimpleLoadBalancer(NewRoundRobinStrategy(), &cfg.LoadBalancer, nil)
	lb.UpdateNodes([]*discovery.NodeInfo{
		{
			ID:      "test-node",
			Name:    "test-node",
			Address: address,
			Status:  discovery.NodeStatusActive,
			Tasks:   []*pb.IOTask{{Name: "test_task"}},
		},
	})

	return &LumenClient{
		config:   cfg,
		pool:     NewGRPCConnectionPool(&PoolConfig{MaxIdleTime: time.Minute, MaxLifetime: time.Minute, HealthCheck: false}, nil),
		balancer: lb,
		logger:   ensureLogger(nil),
		metrics:  &ClientMetrics{LastUpdated: time.Now()},
	}
}

func clientResponseChunk(seq, total, offset uint64, result string, final bool) *pb.InferResponse {
	return &pb.InferResponse{
		CorrelationId: "corr-1",
		IsFinal:       final,
		Result:        []byte(result),
		Meta: map[string]string{
			types.MetaOutputKind: types.OutputKindRaw,
		},
		Seq:        seq,
		Total:      total,
		Offset:     offset,
		ResultMime: types.DefaultTensorMIME,
	}
}
