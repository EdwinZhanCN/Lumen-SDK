package client

import (
	"context"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/edwinzhancn/lumen-sdk/pkg/config"
	"github.com/edwinzhancn/lumen-sdk/pkg/discovery"
	"github.com/edwinzhancn/lumen-sdk/pkg/types"
	pb "github.com/edwinzhancn/lumen-sdk/proto"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestTensorContractCapabilitiesFromGRPC(t *testing.T) {
	client, _ := newTensorContractClient(t, tensorContractCapabilities(types.PreprocessSigLIP2BasePatch16_224Image))

	assertContract := func(taskName, serviceName, preprocessID string, batching bool) {
		t.Helper()
		contract, gotService, ok := client.FindTaskContract(taskName)
		if !ok {
			t.Fatalf("FindTaskContract(%q) did not find a task", taskName)
		}
		if gotService != serviceName {
			t.Fatalf("FindTaskContract(%q) service = %q, want %q", taskName, gotService, serviceName)
		}
		if contract.TensorPreprocessID() != preprocessID {
			t.Fatalf("FindTaskContract(%q) preprocess id = %q, want %q", taskName, contract.TensorPreprocessID(), preprocessID)
		}
		if contract.TensorBatchingSupported() != batching {
			t.Fatalf("FindTaskContract(%q) batching = %v, want %v", taskName, contract.TensorBatchingSupported(), batching)
		}
	}

	assertContract(types.TaskSemanticTextEmbed, types.ServiceSigLIP, "", false)
	assertContract(types.TaskSemanticImageEmbed, types.ServiceSigLIP, types.PreprocessSigLIP2BasePatch16_224Image, true)
	assertContract(types.TaskBioCLIPClassify, types.ServiceBioCLIP, types.PreprocessBioCLIP224Image, true)
	assertContract(types.TaskOCR, types.ServiceOCR, "", false)
	assertContract(types.TaskFaceRecognition, types.ServiceFace, "", false)

	for _, node := range client.GetNodes() {
		for _, capability := range node.Capabilities {
			if capability.GetProtocolVersion() != "1.0" {
				t.Fatalf("capability %q protocol_version = %q, want 1.0", capability.GetServiceName(), capability.GetProtocolVersion())
			}
		}
	}
}

func TestTensorContractKnownPreprocessUsesTensorRequestOverGRPC(t *testing.T) {
	client, server := newTensorContractClient(t, tensorContractCapabilities(types.PreprocessSigLIP2BasePatch16_224Image))

	contract, serviceName, ok := client.FindTaskContract(types.TaskSemanticImageEmbed)
	if !ok {
		t.Fatalf("semantic image task contract not found")
	}
	preprocessor, ok := types.DefaultTensorPreprocessorRegistry().Lookup(contract.TensorPreprocessID())
	if !ok {
		t.Fatalf("known preprocess id %q was not registered", contract.TensorPreprocessID())
	}

	tensor, err := preprocessor.Preprocess(context.Background(), types.ImageInput{
		Data:       make([]byte, 224*224*3),
		Width:      224,
		Height:     224,
		Channels:   3,
		Layout:     "HWC",
		DType:      "uint8",
		ColorSpace: "RGB",
	})
	if err != nil {
		t.Fatalf("Preprocess() error = %v", err)
	}

	req := types.NewInferRequest(types.TaskSemanticImageEmbed).
		WithCorrelationID("known-tensor-path").
		ForTensorInput(tensor.Payload, tensor.PayloadMIME, tensor.Descriptor).
		WithService(serviceName).
		Build()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := client.Infer(ctx, req)
	if err != nil {
		t.Fatalf("Infer(tensor) error = %v", err)
	}
	if resp.Meta[types.MetaInputKind] != types.InputKindTensor {
		t.Fatalf("response observed input kind = %q, want tensor", resp.Meta[types.MetaInputKind])
	}
	if resp.Meta["observed_preprocess_id"] != types.PreprocessSigLIP2BasePatch16_224Image {
		t.Fatalf("response observed preprocess id = %q", resp.Meta["observed_preprocess_id"])
	}

	seen := server.mustRecv(t)
	if seen.PayloadMime != types.DefaultTensorMIME {
		t.Fatalf("server saw payload_mime = %q, want %q", seen.PayloadMime, types.DefaultTensorMIME)
	}
	if seen.Meta[types.MetaPreprocessID] != types.PreprocessSigLIP2BasePatch16_224Image {
		t.Fatalf("server saw preprocess id = %q", seen.Meta[types.MetaPreprocessID])
	}
}

func TestTensorContractUnknownPreprocessFallsBackToRawOverGRPC(t *testing.T) {
	const futurePreprocessID = "future_siglip_unknown_v99"
	client, server := newTensorContractClient(t, tensorContractCapabilities(futurePreprocessID))

	contract, serviceName, ok := client.FindTaskContract(types.TaskSemanticImageEmbed)
	if !ok {
		t.Fatalf("semantic image task contract not found")
	}
	if !contract.HasTensorPath() {
		t.Fatalf("expected node to advertise a tensor path for version-skew test")
	}
	if _, ok := types.DefaultTensorPreprocessorRegistry().Lookup(contract.TensorPreprocessID()); ok {
		t.Fatalf("future preprocess id %q should not be registered", contract.TensorPreprocessID())
	}

	// Mandatory fallback rule: unknown node tensor preprocess IDs indicate node/SDK
	// version skew and should degrade to the raw path at high-level callers.
	req := types.NewInferRequest(types.TaskSemanticImageEmbed).
		WithCorrelationID("unknown-preprocess-raw-fallback").
		ForSemanticImageEmbed([]byte("fake-jpeg-bytes"), "image/jpeg").
		WithService(serviceName).
		Build()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := client.Infer(ctx, req)
	if err != nil {
		t.Fatalf("Infer(raw fallback) error = %v", err)
	}
	if resp.Meta[types.MetaInputKind] != types.InputKindRaw {
		t.Fatalf("response observed input kind = %q, want raw", resp.Meta[types.MetaInputKind])
	}
	if resp.Meta["observed_preprocess_id"] != "" {
		t.Fatalf("raw fallback should not send preprocess id, got %q", resp.Meta["observed_preprocess_id"])
	}

	seen := server.mustRecv(t)
	if seen.PayloadMime != "image/jpeg" {
		t.Fatalf("server saw payload_mime = %q, want image/jpeg", seen.PayloadMime)
	}
	if seen.Meta[types.MetaInputKind] == types.InputKindTensor {
		t.Fatalf("server saw tensor metadata during raw fallback: %#v", seen.Meta)
	}
}

func newTensorContractClient(t *testing.T, capabilities []*pb.Capability) (*LumenClient, *tensorContractServer) {
	t.Helper()

	addr, server := startTensorContractServer(t, capabilities)
	host, port, err := splitEndpoint(addr)
	if err != nil {
		t.Fatalf("split endpoint: %v", err)
	}

	resolver := &fakeNodeResolver{events: []discovery.NodeEvent{{
		Type: discovery.NodeDiscovered,
		Resolved: discovery.ResolvedNode{
			Identity:  discovery.NewNodeIdentity("local", "tensor-contract-node"),
			Addresses: []string{host},
			Port:      port,
			Txt:       map[string]string{"tasks": strings.Join(capabilityTaskNames(capabilities), ",")},
		},
	}}}

	cfg := config.DefaultConfig()
	cfg.Discovery.ConnectTimeout = 2 * time.Second
	cfg.Discovery.RediscoveryBackoffMin = 100 * time.Millisecond
	cfg.Discovery.RediscoveryBackoffMax = time.Second

	client := &LumenClient{
		pool:     NewPoolWithOptions(zap.NewNop(), PoolOptions{ConnectTimeout: cfg.Discovery.ConnectTimeout}),
		resolver: resolver,
		config:   cfg,
		logger:   zap.NewNop(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	if err := client.Start(ctx); err != nil {
		t.Fatalf("client.Start() error = %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	waitUntil(t, func() bool {
		for _, node := range client.GetNodes() {
			if node.IsActive() && len(node.Capabilities) > 0 {
				return true
			}
		}
		return false
	})

	return client, server
}

func startTensorContractServer(t *testing.T, capabilities []*pb.Capability) (string, *tensorContractServer) {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := grpc.NewServer()
	contractServer := &tensorContractServer{
		capabilities: capabilities,
		seen:         make(chan *pb.InferRequest, 16),
	}
	pb.RegisterInferenceServer(server, contractServer)
	go func() {
		_ = server.Serve(lis)
	}()
	t.Cleanup(func() {
		server.Stop()
		_ = lis.Close()
	})
	return lis.Addr().String(), contractServer
}

type tensorContractServer struct {
	pb.UnimplementedInferenceServer
	capabilities []*pb.Capability
	seen         chan *pb.InferRequest
}

func (s *tensorContractServer) GetCapabilities(context.Context, *emptypb.Empty) (*pb.Capability, error) {
	if len(s.capabilities) == 0 {
		return &pb.Capability{}, nil
	}
	return s.capabilities[0], nil
}

func (s *tensorContractServer) StreamCapabilities(_ *emptypb.Empty, stream grpc.ServerStreamingServer[pb.Capability]) error {
	for _, capability := range s.capabilities {
		if err := stream.Send(capability); err != nil {
			return err
		}
	}
	return nil
}

func (s *tensorContractServer) Health(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (s *tensorContractServer) Infer(stream grpc.BidiStreamingServer[pb.InferRequest, pb.InferResponse]) error {
	assembled, err := recvInferRequest(stream)
	if err != nil {
		return err
	}

	select {
	case s.seen <- assembled:
	default:
	}

	inputKind := types.InputKindRaw
	if assembled.Meta[types.MetaInputKind] == types.InputKindTensor {
		inputKind = types.InputKindTensor
	}
	return stream.Send(&pb.InferResponse{
		CorrelationId: assembled.CorrelationId,
		IsFinal:       true,
		Result:        []byte(`{"ok":true}`),
		ResultMime:    "application/json",
		Meta: map[string]string{
			types.MetaInputKind:      inputKind,
			"observed_payload_mime":  assembled.PayloadMime,
			"observed_preprocess_id": assembled.Meta[types.MetaPreprocessID],
			"observed_service":       assembled.Meta[types.MetaService],
		},
	})
}

func (s *tensorContractServer) mustRecv(t *testing.T) *pb.InferRequest {
	t.Helper()
	select {
	case req := <-s.seen:
		return req
	case <-time.After(5 * time.Second):
		t.Fatalf("server did not receive Infer request")
		return nil
	}
}

func recvInferRequest(stream grpc.BidiStreamingServer[pb.InferRequest, pb.InferResponse]) (*pb.InferRequest, error) {
	var assembled *pb.InferRequest
	var payload []byte
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if assembled == nil {
			assembled = &pb.InferRequest{
				CorrelationId: req.GetCorrelationId(),
				Task:          req.GetTask(),
				PayloadMime:   req.GetPayloadMime(),
				Seq:           req.GetSeq(),
				Total:         req.GetTotal(),
				Offset:        req.GetOffset(),
				Meta:          copyStringMap(req.GetMeta()),
			}
		}
		payload = append(payload, req.Payload...)
	}
	if assembled == nil {
		assembled = &pb.InferRequest{Meta: map[string]string{}}
	}
	assembled.Payload = payload
	return assembled, nil
}

func tensorContractCapabilities(siglipImagePreprocessID string) []*pb.Capability {
	return []*pb.Capability{
		{
			ServiceName:     types.ServiceSigLIP,
			Runtime:         "test-runtime",
			ProtocolVersion: "1.0",
			Tasks: []*pb.IOTask{
				{
					Name:                    types.TaskSemanticTextEmbed,
					InputMimes:              []string{"text/plain"},
					OutputMimes:             []string{"application/json;schema=embedding_v1"},
					TensorPreprocessId:      "",
					TensorBatchingSupported: false,
				},
				{
					Name:                    types.TaskSemanticImageEmbed,
					InputMimes:              []string{"image/jpeg", "image/png", types.DefaultTensorMIME},
					OutputMimes:             []string{"application/json;schema=embedding_v1"},
					TensorPreprocessId:      siglipImagePreprocessID,
					TensorBatchingSupported: true,
				},
			},
		},
		{
			ServiceName:     types.ServiceBioCLIP,
			Runtime:         "test-runtime",
			ProtocolVersion: "1.0",
			Tasks: []*pb.IOTask{{
				Name:                    types.TaskBioCLIPClassify,
				InputMimes:              []string{"image/jpeg", "image/png", types.DefaultTensorMIME},
				OutputMimes:             []string{"application/json;schema=labels_v1"},
				TensorPreprocessId:      types.PreprocessBioCLIP224Image,
				TensorBatchingSupported: true,
			}},
		},
		{
			ServiceName:     types.ServiceOCR,
			Runtime:         "test-runtime",
			ProtocolVersion: "1.0",
			Tasks: []*pb.IOTask{{
				Name:                    types.TaskOCR,
				InputMimes:              []string{"image/jpeg", "image/png"},
				OutputMimes:             []string{"application/json;schema=ocr_v1"},
				TensorPreprocessId:      "",
				TensorBatchingSupported: false,
			}},
		},
		{
			ServiceName:     types.ServiceFace,
			Runtime:         "test-runtime",
			ProtocolVersion: "1.0",
			Tasks: []*pb.IOTask{{
				Name:                    types.TaskFaceRecognition,
				InputMimes:              []string{"image/jpeg", "image/png"},
				OutputMimes:             []string{"application/json;schema=face_v1"},
				TensorPreprocessId:      "",
				TensorBatchingSupported: false,
			}},
		},
	}
}

func capabilityTaskNames(capabilities []*pb.Capability) []string {
	seen := make(map[string]bool)
	var names []string
	for _, capability := range capabilities {
		for _, task := range capability.GetTasks() {
			name := strings.TrimSpace(task.GetName())
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			names = append(names, name)
		}
	}
	return names
}
