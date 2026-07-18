package client

// Contract test for Lumen Hub's control-plane-first startup: the hub binds its
// gRPC port before models are downloaded/loaded, so a freshly discovered node
// is reachable (TCP + in-band Health OK) while Infer / GetCapabilities /
// StreamCapabilities answer UNAVAILABLE, potentially for many minutes. The SDK
// must keep the node, keep retrying the capability fetch while the connection
// stays Ready, and become fully functional once the hub flips to ready —
// without any reconnect.

import (
	"context"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/edwinzhancn/lumen-sdk/pkg/config"
	"github.com/edwinzhancn/lumen-sdk/pkg/discovery"
	"github.com/edwinzhancn/lumen-sdk/pkg/types"
	pb "github.com/edwinzhancn/lumen-sdk/proto"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// startingHubServer mirrors the hub's LazyInference gate: UNAVAILABLE on the
// data plane until ready is flipped, in-band Health reachable throughout.
type startingHubServer struct {
	pb.UnimplementedInferenceServer
	inner *tensorContractServer
	ready atomic.Bool
}

func (s *startingHubServer) gate() error {
	if s.ready.Load() {
		return nil
	}
	return status.Error(codes.Unavailable, "lumen hub is starting; inference is not ready yet")
}

func (s *startingHubServer) GetCapabilities(ctx context.Context, empty *emptypb.Empty) (*pb.Capability, error) {
	if err := s.gate(); err != nil {
		return nil, err
	}
	return s.inner.GetCapabilities(ctx, empty)
}

func (s *startingHubServer) StreamCapabilities(empty *emptypb.Empty, stream grpc.ServerStreamingServer[pb.Capability]) error {
	if err := s.gate(); err != nil {
		return err
	}
	return s.inner.StreamCapabilities(empty, stream)
}

func (s *startingHubServer) Health(ctx context.Context, empty *emptypb.Empty) (*emptypb.Empty, error) {
	return s.inner.Health(ctx, empty)
}

func (s *startingHubServer) Infer(stream grpc.BidiStreamingServer[pb.InferRequest, pb.InferResponse]) error {
	if err := s.gate(); err != nil {
		return err
	}
	return s.inner.Infer(stream)
}

func TestHubControlPlaneFirstStartupRecoversWithoutReconnect(t *testing.T) {
	capabilities := tensorContractCapabilities(types.PreprocessSigLIP2BasePatch16_224Image)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := grpc.NewServer()
	hub := &startingHubServer{inner: &tensorContractServer{
		capabilities: capabilities,
		seen:         make(chan *pb.InferRequest, 16),
	}}
	pb.RegisterInferenceServer(server, hub)
	go func() { _ = server.Serve(lis) }()
	t.Cleanup(func() {
		server.Stop()
		_ = lis.Close()
	})

	host, port, err := splitEndpoint(lis.Addr().String())
	if err != nil {
		t.Fatalf("split endpoint: %v", err)
	}
	resolver := &fakeNodeResolver{events: []discovery.NodeEvent{{
		Type: discovery.NodeDiscovered,
		Resolved: discovery.ResolvedNode{
			Identity:  discovery.NewNodeIdentity("local", "starting-hub-node"),
			Addresses: []string{host},
			Port:      port,
			Txt:       map[string]string{"tasks": strings.Join(capabilityTaskNames(capabilities), ",")},
		},
	}}}

	cfg := config.DefaultConfig()
	cfg.Discovery.ConnectTimeout = 2 * time.Second
	client := &LumenClient{
		pool:     NewPoolWithOptions(zap.NewNop(), PoolOptions{ConnectTimeout: cfg.Discovery.ConnectTimeout}),
		resolver: resolver,
		config:   cfg,
		logger:   zap.NewNop(),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	if err := client.Start(ctx); err != nil {
		t.Fatalf("client.Start() against a starting hub must not fail, got %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	// While starting: the node must not gain capabilities, and no task
	// contract may be visible.
	time.Sleep(1500 * time.Millisecond)
	if _, _, ok := client.FindTaskContract(types.TaskSemanticTextEmbed); ok {
		t.Fatalf("task contract visible while the hub is still starting")
	}

	// Hub finishes downloading/loading/warmup.
	hub.ready.Store(true)

	// The SDK must recover on the same connection: capability retry keeps
	// running while the SubConn stays Ready (control-plane-first hubs stay
	// unready for minutes; the fetch loop must not give up).
	deadline := time.Now().Add(20 * time.Second)
	for {
		if _, _, ok := client.FindTaskContract(types.TaskSemanticTextEmbed); ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("capabilities never appeared after the hub became ready")
		}
		time.Sleep(100 * time.Millisecond)
	}

	response, err := client.Infer(ctx, &pb.InferRequest{
		CorrelationId: "startup-contract",
		Task:          types.TaskSemanticTextEmbed,
		Payload:       []byte("hello"),
		PayloadMime:   "text/plain",
	})
	if err != nil {
		t.Fatalf("Infer() after hub became ready: %v", err)
	}
	if !response.GetIsFinal() {
		t.Fatalf("Infer() response not final: %+v", response)
	}
}
