package client_test

import (
	"context"
	"testing"
	"time"

	"github.com/edwinzhancn/lumen-sdk/pkg/client"
	"github.com/edwinzhancn/lumen-sdk/pkg/config"
	"github.com/edwinzhancn/lumen-sdk/pkg/types"
	"go.uber.org/zap"
	"google.golang.org/grpc/status"
)

// TestSemanticTextEmbedOnly focuses purely on semantic_text_embed with
// detailed per-step diagnostics.
func TestSemanticTextEmbedOnly(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	lumenClient, err := client.NewLumenClient(config.DefaultConfig(), logger)
	if err != nil {
		t.Fatalf("NewLumenClient: %v", err)
	}
	defer lumenClient.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := lumenClient.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Wait for nodes
	t.Log("Waiting for nodes...")
	var found bool
	for i := 0; i < 30; i++ {
		time.Sleep(500 * time.Millisecond)
		if len(lumenClient.GetNodes()) > 0 {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("No nodes discovered")
	}

	stats := lumenClient.PoolStats()
	t.Logf("Pool: total=%d healthy=%d", stats.TotalConnections, stats.HealthyConnections)
	for _, n := range lumenClient.GetNodes() {
		names := make([]string, len(n.Tasks))
		for i, tk := range n.Tasks {
			names[i] = tk.Name
		}
		t.Logf("Node: id=%s status=%s active=%v tasks=%v", n.ID, n.Status, n.IsActive(), names)
	}

	if stats.HealthyConnections == 0 {
		t.Fatal("No healthy nodes — cannot test")
	}

	// Build request — exactly like Lumilio's lumen_service.go
	req := types.NewInferRequest("semantic_text_embed").
		ForSemanticTextEmbed("hello world", types.ServiceSigLIP).
		WithCorrelationID("test-semtext").
		Build()

	t.Logf("Request: task=%s mime=%s payload=%d meta=%v",
		req.Task, req.PayloadMime, len(req.Payload), req.Meta)

	// Validate
	if err := types.ValidateTaskRequest(req); err != nil {
		t.Fatalf("ValidateTaskRequest: %v", err)
	}
	t.Log("ValidateTaskRequest: OK")

	// Infer
	callCtx, callCancel := context.WithTimeout(ctx, 15*time.Second)
	defer callCancel()

	t.Log("Calling Infer...")
	start := time.Now()
	resp, err := lumenClient.Infer(callCtx, req)
	elapsed := time.Since(start)

	if err != nil {
		st, _ := status.FromError(err)
		t.Logf("Infer FAILED after %v", elapsed)
		t.Logf("  error: %v", err)
		if st != nil {
			t.Logf("  gRPC code: %s (%d), message: %s", st.Code(), st.Code(), st.Message())
		}
	} else {
		t.Logf("Infer OK after %v", elapsed.Round(time.Millisecond))
		t.Logf("  result_mime=%s result_len=%d", resp.ResultMime, len(resp.Result))

		embedResp, err := types.ParseInferResponse(resp).AsEmbeddingResponse()
		if err != nil {
			t.Logf("Parse embedding: %v", err)
		} else {
			t.Logf("  model=%s dim=%d", embedResp.ModelID, embedResp.DimValue())
		}
	}

	// Pool after
	stats2 := lumenClient.PoolStats()
	t.Logf("Pool after: total=%d healthy=%d", stats2.TotalConnections, stats2.HealthyConnections)
	if stats2.HealthyConnections == 0 {
		t.Error("Node was kicked out of healthy list!")
	}
}
