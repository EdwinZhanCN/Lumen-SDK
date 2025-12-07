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

	const FaceDetectTask = "face_detect"

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

	fmt.Println("Testing face detection...")

	// Get image list from environment variable
	testImages := getImageList()

	for _, filename := range testImages {
		testFaceDetection(ctx, lumenClient, filename, FaceDetectTask)
	}

	fmt.Println("\nFace detection tests completed!")
}

// getImageList returns image files from environment variable
func getImageList() []string {
	images := os.Getenv("DETECT_IMAGES")
	if images == "" {
		fmt.Println("Error: DETECT_IMAGES environment variable not set")
		fmt.Println("Usage: DETECT_IMAGES=\"image1.jpg,image2.png,image3.jpeg\" go run main.go")
		os.Exit(1)
	}
	return strings.Split(images, ",")
}

func testFaceDetection(ctx context.Context, lumenClient *client.LumenClient, filename string, faceDetectTask string) {
	// Load image file
	imageData, err := os.ReadFile(filename)
	if err != nil {
		fmt.Printf("Failed to load %s: %v\n", filename, err)
		return
	}

	fmt.Printf("Testing %s (%d bytes)\n", filename, len(imageData))

	// Create face detection request with default parameters
	faceReq, err := types.NewFaceRecognitionRequest(imageData)
	if err != nil {
		fmt.Printf("Failed to create face detection request: %v\n", err)
		return
	}

	inferReq := types.NewInferRequest(faceDetectTask).
		WithCorrelationID("face_detect_test").
		ForFaceDetection(faceReq, faceDetectTask).
		Build()

	// Perform face detection with retry
	resp, err := lumenClient.InferWithRetry(ctx, inferReq,
		client.WithMaxWaitTime(30*time.Second),
		client.WithRetryInterval(3*time.Second),
		client.WithWaitForTask(true))

	if err != nil {
		fmt.Printf("Face detection failed: %v\n", err)
		return
	}

	// Parse and display results
	faceResp, err := types.ParseInferResponse(resp).AsFaceResponse()
	rawResp := types.ParseInferResponse(resp).Raw()
	if err != nil {
		fmt.Printf("Failed to parse response: %v\n, raw response: %s", err, rawResp)
		return
	}

	fmt.Printf("Success! Detected %d faces:\n", faceResp.Count)
	fmt.Printf("   Model: %s\n", faceResp.ModelID)

	for i, face := range faceResp.Faces {
		fmt.Printf("   Face %d: confidence=%.2f, bbox=[%.1f,%.1f,%.1f,%.1f]\n",
			i+1, face.Confidence,
			face.BBox[0], face.BBox[1], face.BBox[2], face.BBox[3])

		// Show landmarks if available
		if len(face.Landmarks) > 0 {
			fmt.Printf("             landmarks: %d points\n", len(face.Landmarks)/2)
		}
	}
}
