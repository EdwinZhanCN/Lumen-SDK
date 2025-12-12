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

	const OCRTask = "ocr"

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

	fmt.Println("Testing OCR...")

	// Get image list from environment variable
	testImages := getImageList()

	for _, filename := range testImages {
		testOCR(ctx, lumenClient, filename, OCRTask)
	}

	fmt.Println("\nOCR tests completed!")
}

// getImageList returns image files from environment variable
func getImageList() []string {
	images := os.Getenv("OCR_IMAGES")
	if images == "" {
		fmt.Println("Error: OCR_IMAGES environment variable not set")
		fmt.Println("Usage: OCR_IMAGES=\"doc1.jpg,doc2.png\" go run main.go")
		os.Exit(1)
	}
	return strings.Split(images, ",")
}

func testOCR(ctx context.Context, lumenClient *client.LumenClient, filename string, ocrTask string) {
	// Load image file
	imageData, err := os.ReadFile(filename)
	if err != nil {
		fmt.Printf("Failed to load %s: %v\n", filename, err)
		return
	}

	fmt.Printf("Testing %s (%d bytes)\n", filename, len(imageData))

	// Create OCR request with default parameters
	ocrReq, err := types.NewOCRRequest(imageData)
	if err != nil {
		fmt.Printf("Failed to create OCR request: %v\n", err)
		return
	}

	inferReq := types.NewInferRequest(ocrTask).
		WithCorrelationID("ocr_test").
		ForOCR(ocrReq, ocrTask).
		Build()

	// Perform OCR with retry
	resp, err := lumenClient.InferWithRetry(ctx, inferReq,
		client.WithMaxWaitTime(30*time.Second),
		client.WithRetryInterval(3*time.Second),
		client.WithWaitForTask(true))

	if err != nil {
		fmt.Printf("OCR failed: %v\n", err)
		return
	}

	// Parse and display results
	ocrResp, err := types.ParseInferResponse(resp).AsOCRResponse()
	rawResp := types.ParseInferResponse(resp).Raw()
	if err != nil {
		fmt.Printf("Failed to parse response: %v\n, raw response: %s", err, rawResp)
		return
	}

	fmt.Printf("Success! Detected %d text regions:\n", ocrResp.Count)
	fmt.Printf("   Model: %s\n", ocrResp.ModelID)

	for i, item := range ocrResp.Items {
		fmt.Printf("   Item %d: confidence=%.2f, text=\"%s\"\n",
			i+1, item.Confidence, item.Text)
		fmt.Printf("           box=%v\n", item.Box)
	}
}
