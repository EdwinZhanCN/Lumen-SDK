package api

import (
	"context"
	"fmt"

	"github.com/edwinzhancn/lumen-sdk/pkg/client"
	"github.com/edwinzhancn/lumen-sdk/pkg/config"
	"github.com/edwinzhancn/lumen-sdk/pkg/types"
	"go.uber.org/zap"
)

func main() {
	// Create a new logger instance and pass it to the client
	logger, _ := zap.NewProduction()
	lumenClientConfig := config.DefaultConfig()
	lumenClient, err := client.NewLumenClient(lumenClientConfig, logger)
	if err != nil {
		panic(err)
	}
	// Start browsing the lumen node use mDNS discovery
	ctx := context.Background()
	if err := lumenClient.Start(ctx); err != nil {
		panic(err)
	}
	// Create some new lumen_clip requests
	textData := []byte("cat")
	embeddingReq, err := types.NewEmbeddingRequest(textData)
	if err != nil {
		panic(err)
	}

	inferReq := types.NewInferRequest("lumen_clip_embed").
		WithCorrelationID("my_text_request_123").
		ForEmbedding(embeddingReq, "lumen_clip_embed").
		Build()

	resp, err := lumenClient.Infer(ctx, inferReq)
	if err != nil {
		logger.Error("failed to infer", zap.Error(err))
	}

	embeddingResp, err := types.ParseInferResponse(resp).
		AsEmbeddingResponse()
	if err == nil {
		logger.Error("failed to parse embedding response", zap.Error(err))
	}

	// print embeddingResp
	fmt.Println(embeddingResp)
}
