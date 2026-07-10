//go:build realmodel

package client

import (
	"context"
	"encoding/json"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/edwinzhancn/lumen-sdk/pkg/config"
	"github.com/edwinzhancn/lumen-sdk/pkg/discovery"
	"github.com/edwinzhancn/lumen-sdk/pkg/types"
	pb "github.com/edwinzhancn/lumen-sdk/proto"

	"go.uber.org/zap"
)

const (
	siglipConformanceAddrEnv       = "LUMEN_SIGLIP_CONFORMANCE_ADDR"
	siglipConformanceImageEnv      = "LUMEN_SIGLIP_CONFORMANCE_IMAGE"
	siglipConformanceMinCosineEnv  = "LUMEN_SIGLIP_CONFORMANCE_MIN_COSINE"
	siglipConformanceMaxAbsDiffEnv = "LUMEN_SIGLIP_CONFORMANCE_MAX_ABS_DIFF"
)

// TestSigLIPRealModelTensorConformance compares the SDK-produced tensor fast
// path against node-owned raw-image preprocessing on a real SigLIP model.
//
// This is intentionally opt-in because it requires a running Lumen inference node
// with real SigLIP weights loaded. Example:
//
//	LUMEN_SIGLIP_CONFORMANCE_ADDR=127.0.0.1:50051 \
//	go test -tags=realmodel ./pkg/client -run TestSigLIPRealModelTensorConformance -v
//
// By default this uses the SDK-local fixture at testdata/siglip/bus.jpg. Set
// LUMEN_SIGLIP_CONFORMANCE_IMAGE to run the same conformance check on another
// explicit JPEG fixture.
func TestSigLIPRealModelTensorConformance(t *testing.T) {
	addr := strings.TrimSpace(os.Getenv(siglipConformanceAddrEnv))
	if addr == "" {
		t.Skipf("set %s to a real Lumen gRPC endpoint", siglipConformanceAddrEnv)
	}

	imagePath := resolveSiglipConformanceImage(t)
	imageBytes, err := os.ReadFile(imagePath)
	if err != nil {
		t.Fatalf("read conformance image %q: %v", imagePath, err)
	}

	client := newStaticRealModelClient(t, addr)
	contract, serviceName, ok := client.FindTaskContract(types.TaskSemanticImageEmbed)
	if !ok {
		t.Fatalf("node at %s does not advertise task %q", addr, types.TaskSemanticImageEmbed)
	}
	if serviceName != types.ServiceSigLIP {
		t.Fatalf("task %q resolved to service %q, want %q", types.TaskSemanticImageEmbed, serviceName, types.ServiceSigLIP)
	}
	if !contract.HasTensorPath() {
		t.Fatalf("node SigLIP task does not advertise a tensor path")
	}

	preprocessor, ok := types.DefaultTensorPreprocessorRegistry().Lookup(contract.TensorPreprocessID())
	if !ok {
		t.Fatalf("SDK registry does not know node SigLIP preprocess id %q", contract.TensorPreprocessID())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	rawReq := types.NewInferRequest(types.TaskSemanticImageEmbed).
		WithCorrelationID("siglip-realmodel-raw").
		ForSemanticImageEmbed(imageBytes, "image/jpeg").
		WithService(serviceName).
		Build()
	rawResp, err := client.Infer(ctx, rawReq)
	if err != nil {
		t.Fatalf("raw SigLIP Infer() error: %v", err)
	}
	rawEmbedding := parseEmbeddingResponse(t, rawResp)

	tensor, err := preprocessor.Preprocess(ctx, types.ImageInput{
		Encoded:     imageBytes,
		PayloadMIME: "image/jpeg",
	})
	if err != nil {
		t.Fatalf("SDK tensor preprocessor %q error: %v", preprocessor.ID(), err)
	}
	tensorReq := types.NewInferRequest(types.TaskSemanticImageEmbed).
		WithCorrelationID("siglip-realmodel-tensor").
		ForTensorInput(tensor.Payload, tensor.PayloadMIME, tensor.Descriptor).
		WithService(serviceName).
		Build()
	tensorResp, err := client.Infer(ctx, tensorReq)
	if err != nil {
		t.Fatalf("tensor SigLIP Infer() error: %v", err)
	}
	tensorEmbedding := parseEmbeddingResponse(t, tensorResp)

	if rawEmbedding.DimValue() == 0 || tensorEmbedding.DimValue() == 0 {
		t.Fatalf("empty embeddings: raw dim=%d tensor dim=%d", rawEmbedding.DimValue(), tensorEmbedding.DimValue())
	}
	if rawEmbedding.DimValue() != tensorEmbedding.DimValue() {
		t.Fatalf("embedding dim mismatch: raw=%d tensor=%d", rawEmbedding.DimValue(), tensorEmbedding.DimValue())
	}

	cosine := cosineSimilarity(rawEmbedding.Vector, tensorEmbedding.Vector)
	maxDiff := maxAbsDiff(rawEmbedding.Vector, tensorEmbedding.Vector)
	// Keep this strict: the SDK tensor preprocessor should match node raw
	// preprocessing closely enough that the same model produces near-identical
	// embeddings. Environment variables can loosen or tighten this for backend
	// experiments.
	minCosine := floatEnv(siglipConformanceMinCosineEnv, 0.999)
	maxAllowedDiff := floatEnv(siglipConformanceMaxAbsDiffEnv, 0.005)

	t.Logf("SigLIP raw-vs-tensor: preprocess_id=%s dim=%d cosine=%.8f max_abs_diff=%.8f image=%s",
		contract.TensorPreprocessID(), rawEmbedding.DimValue(), cosine, maxDiff, imagePath)

	if cosine < minCosine {
		t.Fatalf("SigLIP raw-vs-tensor cosine %.8f < %.8f", cosine, minCosine)
	}
	if maxDiff > maxAllowedDiff {
		t.Fatalf("SigLIP raw-vs-tensor max_abs_diff %.8f > %.8f", maxDiff, maxAllowedDiff)
	}
}

func newStaticRealModelClient(t *testing.T, endpoint string) *LumenClient {
	t.Helper()

	host, port, err := splitEndpoint(normalizeEndpoint(endpoint))
	if err != nil {
		t.Fatalf("parse node endpoint %q: %v", endpoint, err)
	}

	resolver := &fakeNodeResolver{events: []discovery.NodeEvent{{
		Type: discovery.NodeDiscovered,
		Resolved: discovery.ResolvedNode{
			Identity:  discovery.NewNodeIdentity("local", "siglip-realmodel-node"),
			Addresses: []string{host},
			Port:      port,
			Txt:       map[string]string{"tasks": types.TaskSemanticImageEmbed},
		},
	}}}

	cfg := config.DefaultConfig()
	cfg.Discovery.ConnectTimeout = 5 * time.Second
	cfg.Discovery.RediscoveryBackoffMin = 100 * time.Millisecond
	cfg.Discovery.RediscoveryBackoffMax = time.Second

	client := &LumenClient{
		pool:     NewPoolWithOptions(zap.NewNop(), PoolOptions{ConnectTimeout: cfg.Discovery.ConnectTimeout}),
		resolver: resolver,
		config:   cfg,
		logger:   zap.NewNop(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	if err := client.Start(ctx); err != nil {
		t.Fatalf("client.Start() error: %v", err)
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

	return client
}

func normalizeEndpoint(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	if strings.Contains(endpoint, "://") {
		parsed, err := url.Parse(endpoint)
		if err == nil && parsed.Host != "" {
			return parsed.Host
		}
	}
	return endpoint
}

func resolveSiglipConformanceImage(t *testing.T) string {
	t.Helper()
	if configured := strings.TrimSpace(os.Getenv(siglipConformanceImageEnv)); configured != "" {
		return configured
	}

	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("resolve SDK-local SigLIP fixture path: runtime.Caller failed")
	}
	fixturePath := filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "../../testdata/siglip/bus.jpg"))
	if _, err := os.Stat(fixturePath); err != nil {
		t.Fatalf("SDK-local SigLIP fixture is missing at %s: %v", fixturePath, err)
	}
	return fixturePath
}

func parseEmbeddingResponse(t *testing.T, response *pb.InferResponse) types.EmbeddingV1 {
	t.Helper()
	if response == nil {
		t.Fatalf("nil InferResponse")
	}
	var embedding types.EmbeddingV1
	if err := json.Unmarshal(response.Result, &embedding); err != nil {
		t.Fatalf("parse embedding_v1 response: %v; payload=%q", err, string(response.Result))
	}
	return embedding
}

func cosineSimilarity(a, b []float32) float64 {
	var dot, normA, normB float64
	for i := range a {
		av := float64(a[i])
		bv := float64(b[i])
		dot += av * bv
		normA += av * av
		normB += bv * bv
	}
	return dot / (math.Sqrt(normA)*math.Sqrt(normB) + 1e-12)
}

func maxAbsDiff(a, b []float32) float64 {
	var maxDiff float64
	for i := range a {
		diff := math.Abs(float64(a[i] - b[i]))
		if diff > maxDiff {
			maxDiff = diff
		}
	}
	return maxDiff
}

func floatEnv(name string, fallback float64) float64 {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsInf(parsed, 0) || math.IsNaN(parsed) {
		return fallback
	}
	return parsed
}
