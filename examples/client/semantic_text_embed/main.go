package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/edwinzhancn/lumen-sdk/pkg/client"
	"github.com/edwinzhancn/lumen-sdk/pkg/config"
	"github.com/edwinzhancn/lumen-sdk/pkg/types"
	"go.uber.org/zap"
)

// Usage: EMBED_TEXT="hello world" go run main.go
func main() {
	text := os.Getenv("EMBED_TEXT")
	if text == "" {
		fmt.Println("Usage: EMBED_TEXT=\"hello world\" go run main.go")
		os.Exit(1)
	}

	logger, _ := zap.NewProduction()
	defer logger.Sync()

	lumen, err := client.NewLumenClient(config.DefaultConfig(), logger)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer lumen.Close()

	ctx := context.Background()
	if err := lumen.Start(ctx); err != nil {
		log.Fatalf("Failed to start client: %v", err)
	}

	req := types.NewInferRequest(types.TaskSemanticTextEmbed).
		WithCorrelationID("example_text_embed").
		ForSemanticTextEmbed(text).
		Build()

	resp, err := lumen.Infer(ctx, req)
	if err != nil {
		log.Fatalf("Infer failed: %v", err)
	}

	embedding, err := types.ParseInferResponse(resp).AsEmbeddingResponse()
	if err != nil {
		log.Fatalf("Parse failed: %v\nRaw: %s", err, resp.Result)
	}

	fmt.Printf("Text:       %s\n", text)
	fmt.Printf("Model:      %s\n", embedding.ModelID)
	fmt.Printf("Dimensions: %d\n", embedding.DimValue())
	fmt.Printf("Magnitude:  %.4f\n", embedding.Magnitude())

	n := 5
	if len(embedding.Vector) < n {
		n = len(embedding.Vector)
	}
	parts := make([]string, n)
	for i := 0; i < n; i++ {
		parts[i] = fmt.Sprintf("%.4f", embedding.Vector[i])
	}
	fmt.Printf("Vector (first %d): [%s]\n", n, strings.Join(parts, ", "))
}
