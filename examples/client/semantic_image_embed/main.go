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

// Usage: EMBED_IMAGE=photo.jpg go run main.go
func main() {
	imagePath := os.Getenv("EMBED_IMAGE")
	if imagePath == "" {
		fmt.Println("Usage: EMBED_IMAGE=photo.jpg go run main.go")
		os.Exit(1)
	}

	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		log.Fatalf("Failed to read %s: %v", imagePath, err)
	}

	embReq, err := types.NewEmbeddingRequest(imageData)
	if err != nil {
		log.Fatalf("Invalid image: %v", err)
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

	req := types.NewInferRequest(types.TaskSemanticImageEmbed).
		WithCorrelationID("example_image_embed").
		ForSemanticImageEmbed(embReq.Payload, embReq.PayloadMime).
		Build()

	resp, err := lumen.Infer(ctx, req)
	if err != nil {
		log.Fatalf("Infer failed: %v", err)
	}

	embedding, err := types.ParseInferResponse(resp).AsEmbeddingResponse()
	if err != nil {
		log.Fatalf("Parse failed: %v\nRaw: %s", err, resp.Result)
	}

	fmt.Printf("Image:      %s (%s, %d bytes)\n", imagePath, embReq.PayloadMime, len(imageData))
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
