package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/edwinzhancn/lumen-sdk/pkg/client"
	"github.com/edwinzhancn/lumen-sdk/pkg/config"
	"github.com/edwinzhancn/lumen-sdk/pkg/types"
	"go.uber.org/zap"
)

// Usage: FACE_IMAGE=group.jpg go run main.go
func main() {
	imagePath := os.Getenv("FACE_IMAGE")
	if imagePath == "" {
		fmt.Println("Usage: FACE_IMAGE=group.jpg go run main.go")
		os.Exit(1)
	}

	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		log.Fatalf("Failed to read %s: %v", imagePath, err)
	}

	faceReq, err := types.NewFaceRecognitionRequest(imageData)
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

	req := types.NewInferRequest(types.TaskFaceRecognition).
		WithCorrelationID("example_face").
		ForFaceRecognitionRaw(faceReq.Payload, faceReq.PayloadMime).
		Build()

	resp, err := lumen.Infer(ctx, req)
	if err != nil {
		log.Fatalf("Infer failed: %v", err)
	}

	faceResp, err := types.ParseInferResponse(resp).AsFaceResponse()
	if err != nil {
		log.Fatalf("Parse failed: %v\nRaw: %s", err, resp.Result)
	}

	fmt.Printf("Image: %s (%s, %d bytes)\n", imagePath, faceReq.PayloadMime, len(imageData))
	fmt.Printf("Model: %s\n", faceResp.ModelID)
	fmt.Printf("Detected %d faces:\n", faceResp.Count)
	for i, face := range faceResp.Faces {
		fmt.Printf("  %d. confidence=%.2f%% bbox=[%.1f, %.1f, %.1f, %.1f]\n",
			i+1, face.Confidence*100,
			face.BBox[0], face.BBox[1], face.BBox[2], face.BBox[3])
		if len(face.Landmarks) > 0 {
			fmt.Printf("     landmarks: %d points\n", len(face.Landmarks)/2)
		}
		if len(face.Embedding) > 0 {
			fmt.Printf("     embedding: %d dims\n", len(face.Embedding))
		}
	}
}
