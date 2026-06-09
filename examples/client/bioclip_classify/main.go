package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/edwinzhancn/lumen-sdk/pkg/client"
	"github.com/edwinzhancn/lumen-sdk/pkg/config"
	"github.com/edwinzhancn/lumen-sdk/pkg/types"
	"go.uber.org/zap"
)

// Usage: CLASSIFY_IMAGE=animal.jpg go run main.go
//        CLASSIFY_IMAGE=animal.jpg CLASSIFY_TOP_K=3 go run main.go
func main() {
	imagePath := os.Getenv("CLASSIFY_IMAGE")
	if imagePath == "" {
		fmt.Println("Usage: CLASSIFY_IMAGE=animal.jpg go run main.go")
		fmt.Println("       CLASSIFY_IMAGE=animal.jpg CLASSIFY_TOP_K=3 go run main.go")
		os.Exit(1)
	}

	topK := 5
	if v := os.Getenv("CLASSIFY_TOP_K"); v != "" {
		if k, err := strconv.Atoi(v); err == nil && k > 0 {
			topK = k
		}
	}

	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		log.Fatalf("Failed to read %s: %v", imagePath, err)
	}

	classReq, err := types.NewClassificationRequest(imageData)
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

	req := types.NewInferRequest(types.TaskBioCLIPClassify).
		WithCorrelationID("example_bioclip").
		ForBioCLIPClassify(classReq.Payload, classReq.PayloadMime, topK).
		Build()

	resp, err := lumen.Infer(ctx, req)
	if err != nil {
		log.Fatalf("Infer failed: %v", err)
	}

	labels, err := types.ParseInferResponse(resp).AsClassificationResponse()
	if err != nil {
		log.Fatalf("Parse failed: %v\nRaw: %s", err, resp.Result)
	}

	fmt.Printf("Image: %s (%s, %d bytes)\n", imagePath, classReq.PayloadMime, len(imageData))
	fmt.Printf("Model: %s\n", labels.ModelID)
	fmt.Printf("Top %d predictions:\n", topK)
	for i, label := range labels.TopK(topK) {
		fmt.Printf("  %d. %s (%.2f%%)\n", i+1, label.Label, label.Score*100)
	}
}
