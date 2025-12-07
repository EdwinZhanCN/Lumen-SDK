package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/edwinzhancn/lumen-sdk/pkg/client"
	"github.com/edwinzhancn/lumen-sdk/pkg/config"
	"github.com/edwinzhancn/lumen-sdk/pkg/types"
	"go.uber.org/zap"
)

func main() {

	const ClassifyTask = "clip_classify"

	logger, _ := zap.NewProduction()
	defer logger.Sync()

	lumenClientConfig := config.DefaultConfig()
	lumenClient, err := client.NewLumenClient(lumenClientConfig, logger)
	if err != nil {
		log.Fatalf("Failed to create Lumen client: %v", err)
	}
	defer lumenClient.Close()

	ctx := context.Background()
	if err := lumenClient.Start(ctx); err != nil {
		log.Fatalf("Failed to start Lumen client: %v", err)
	}
	fmt.Println("Testing image classification...")

	// Get image list from environment variable or use defaults
	testImages := getImageList()

	for _, filename := range testImages {
		testImageClassification(ctx, lumenClient, filename, ClassifyTask, 1)
	}

	fmt.Println("\nClassification tests completed!")
}

// getImageList returns image files from environment variable
func getImageList() []string {
	images := os.Getenv("CLASSIFY_IMAGES")
	if images == "" {
		fmt.Println("Error: CLASSIFY_IMAGES environment variable not set")
		fmt.Println("Usage: CLASSIFY_IMAGES=\"path1.jpg,path2.png,path3.jpeg\" go run main.go")
		os.Exit(1)
	}
	return strings.Split(images, ",")
}

func testImageClassification(ctx context.Context, lumenClient *client.LumenClient, filename string, classifyTask string, TopK int) {
	// Load image file
	imageData, err := os.ReadFile(filename)
	if err != nil {
		fmt.Printf("Failed to load %s: %v\n", filename, err)
		return
	}

	fmt.Printf("Testing %s (%d bytes)\n", filename, len(imageData))

	// Create classification request
	classificationReq, err := types.NewClassificationRequest(imageData)
	if err != nil {
		fmt.Printf("Failed to create classification request: %v\n", err)
		return
	}

	inferReq := types.NewInferRequest(classifyTask).
		WithCorrelationID("classify_test").
		ForClassification(classificationReq, classifyTask).
		Build()

	// Perform classification with retry
	resp, err := lumenClient.InferWithRetry(ctx, inferReq,
		client.WithMaxWaitTime(30*time.Second),
		client.WithRetryInterval(3*time.Second),
		client.WithWaitForTask(true))

	if err != nil {
		fmt.Printf("Classification failed: %v\n", err)
		return
	}

	// Parse and display results
	classificationResp, err := types.ParseInferResponse(resp).AsClassificationResponse()
	rawResp := types.ParseInferResponse(resp).Raw()
	if err != nil {
		fmt.Printf("Failed to parse response: %v\n, raw response: %s", err, rawResp)
		return
	}

	fmt.Printf("Success! Top %d labels:\n", len(classificationResp.Labels))
	for i, label := range classificationResp.TopK(TopK) {
		fmt.Printf("   %d. %s (%.2f%%)\n", i+1, label.Label, label.Score*100)
	}

	fmt.Printf("\nClassification result:\n%s\n", rawResp)
}
