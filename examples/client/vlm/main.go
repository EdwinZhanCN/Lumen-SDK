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

	const VlmTask = "vlm_generate"

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

	fmt.Println("Testing VLM...")

	// Get image list from environment variable
	testImages := getImageList()

	for _, filename := range testImages {
		testVLM(ctx, lumenClient, filename, VlmTask)
	}

	fmt.Println("\nVLM tests completed!")
}

// getImageList returns image files from environment variable
func getImageList() []string {
	images := os.Getenv("VLM_IMAGES")
	if images == "" {
		fmt.Println("Error: VLM_IMAGES environment variable not set")
		fmt.Println("Usage: VLM_IMAGES=\"img1.jpg,img2.png\" go run main.go")
		os.Exit(1)
	}
	return strings.Split(images, ",")
}

func testVLM(ctx context.Context, lumenClient *client.LumenClient, filename string, vlmTask string) {
	// Load image file
	imageData, err := os.ReadFile(filename)
	if err != nil {
		fmt.Printf("Failed to load %s: %v\n", filename, err)
		return
	}

	fmt.Printf("Testing %s (%d bytes)\n", filename, len(imageData))

	// Create VLM request with default parameters
	vlmReq, err := types.NewImageTextGenerationRequest(imageData,
		types.WithMaxTokens(512),
		// types.WithTemperature(0.2),
		types.WithMessages([]map[string]string{
			{"role": "user", "content": "<image>Describe this image in detail."},
		}))
	if err != nil {
		fmt.Printf("Failed to create VLM request: %v\n", err)
		return
	}

	inferReq := types.NewInferRequest(vlmTask).
		WithCorrelationID("vlm_test").
		ForImageTextGeneration(vlmReq, vlmTask).
		Build()

	// Perform VLM inference with retry
	resp, err := lumenClient.InferWithRetry(ctx, inferReq,
		client.WithMaxWaitTime(120*time.Second),
		client.WithRetryInterval(3*time.Second),
		client.WithWaitForTask(true))

	if err != nil {
		fmt.Printf("VLM inference failed: %v\n", err)
		return
	}

	// Parse and display results
	genResp, err := types.ParseInferResponse(resp).AsTextGenerationResponse()
	rawResp := types.ParseInferResponse(resp).Raw()
	if err != nil {
		fmt.Printf("Failed to parse response: %v\n, raw response: %s", err, rawResp)
		return
	}

	fmt.Printf("Success! Generated text:\n")
	fmt.Printf("   Model: %s\n", genResp.ModelID)
	fmt.Printf("   Generated tokens: %d\n", genResp.GeneratedTokens)
	fmt.Printf("   Input tokens: %d\n", genResp.InputTokens)
	fmt.Printf("   Finish reason: %s\n", genResp.FinishReason)
	fmt.Printf("   Text: \"%s\"\n", genResp.Text)

	if genResp.Metadata != nil {
		fmt.Printf("   Metadata:\n")
		if genResp.Metadata.Temperature > 0 {
			fmt.Printf("     Temperature: %.1f\n", genResp.Metadata.Temperature)
		}
		if genResp.Metadata.TopP > 0 {
			fmt.Printf("     Top P: %.1f\n", genResp.Metadata.TopP)
		}
		if genResp.Metadata.MaxTokens > 0 {
			fmt.Printf("     Max tokens: %d\n", genResp.Metadata.MaxTokens)
		}
		if genResp.Metadata.GenerationTimeMs > 0 {
			fmt.Printf("     Generation time: %.2f ms\n", genResp.Metadata.GenerationTimeMs)
		}
	}

	// Second test with a different prompt
	fmt.Printf("\nTrying with a different prompt...\n")

	vlmReq2, err := types.NewImageTextGenerationRequest(imageData,
		types.WithMaxTokens(200),
		// types.WithTemperature(0.5),
		// types.WithTopP(0.9),
		types.WithPrompt("<image>What objects do you see in this image?"))
	if err != nil {
		fmt.Printf("Failed to create second VLM request: %v\n", err)
		return
	}

	inferReq2 := types.NewInferRequest(vlmTask).
		WithCorrelationID("vlm_test_2").
		ForImageTextGeneration(vlmReq2, vlmTask).
		Build()

	resp2, err := lumenClient.InferWithRetry(ctx, inferReq2,
		client.WithMaxWaitTime(120*time.Second),
		client.WithRetryInterval(3*time.Second),
		client.WithWaitForTask(true))

	if err != nil {
		fmt.Printf("Second VLM inference failed: %v\n", err)
		return
	}

	genResp2, err := types.ParseInferResponse(resp2).AsTextGenerationResponse()
	if err != nil {
		fmt.Printf("Failed to parse second response: %v\n", err)
		return
	}

	fmt.Printf("Second test result:\n")
	fmt.Printf("   Text: \"%s\"\n", genResp2.Text)
	fmt.Printf("   Finish reason: %s\n", genResp2.FinishReason)
	fmt.Printf("   Generated tokens: %d\n", genResp2.GeneratedTokens)
	fmt.Println(resp2)
}
