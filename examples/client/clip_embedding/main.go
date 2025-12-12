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

	const EmbedTask = "clip_text_embed"

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

	fmt.Println("Testing text/image embedding...")

	// Get content list from environment variable
	testContents := getContentList()

	for _, content := range testContents {
		testEmbedding(ctx, lumenClient, content, EmbedTask)
	}

	fmt.Println("\nEmbedding tests completed!")
}

// getContentList returns text or image files from environment variable
func getContentList() []string {
	contents := os.Getenv("EMBED_CONTENTS")
	if contents == "" {
		fmt.Println("Error: EMBED_CONTENTS environment variable not set")
		fmt.Println("Usage for text files: EMBED_CONTENTS=\"text1,text2,text3\" go run main.go")
		fmt.Println("Usage for image files: EMBED_CONTENTS=\"image1.jpg,image2.png,image3.jpeg\" go run main.go")
		os.Exit(1)
	}
	return strings.Split(contents, ",")
}

func testEmbedding(ctx context.Context, lumenClient *client.LumenClient, content string, embedTask string) {
	var payload []byte
	var contentType string

	// Try to read as file first, if fails treat as text content
	if data, err := os.ReadFile(content); err == nil {
		payload = data
		contentType = fmt.Sprintf("file (%s)", content)
	} else {
		// Treat as text content
		payload = []byte(content)
		contentType = fmt.Sprintf("text (%s)", content)
	}

	fmt.Printf("Testing %s (%d bytes)\n", contentType, len(payload))

	// Create embedding request
	embeddingReq, err := types.NewEmbeddingRequest(payload)
	if err != nil {
		fmt.Printf("Failed to create embedding request: %v\n", err)
		return
	}

	inferReq := types.NewInferRequest(embedTask).
		WithCorrelationID("embed_test").
		ForEmbedding(embeddingReq, embedTask).
		Build()

	// Perform embedding with retry
	resp, err := lumenClient.InferWithRetry(ctx, inferReq,
		client.WithMaxWaitTime(30*time.Second),
		client.WithRetryInterval(3*time.Second),
		client.WithWaitForTask(true))

	if err != nil {
		fmt.Printf("Embedding failed: %v\n", err)
		return
	}

	// Parse and display results
	embeddingResp, err := types.ParseInferResponse(resp).AsEmbeddingResponse()
	rawResp := types.ParseInferResponse(resp).Raw()
	if err != nil {
		fmt.Printf("Failed to parse response: %v\n, raw response: %s", err, rawResp)
		return
	}

	fmt.Printf("Success! Embedding details:\n")
	fmt.Printf("   Dimensions: %d\n", embeddingResp.DimValue())
	fmt.Printf("   Model ID: %s\n", embeddingResp.ModelID)
	fmt.Printf("   Magnitude: %.4f\n", embeddingResp.Magnitude())
	fmt.Printf("   First 5 values: [%.4f, %.4f, %.4f, %.4f, %.4f]\n",
		embeddingResp.Vector[0], embeddingResp.Vector[1], embeddingResp.Vector[2],
		embeddingResp.Vector[3], embeddingResp.Vector[4])
}
