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

	const FaceDetectEmbedTask = "face_detect_and_embed"

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

	fmt.Println("Testing face detection and recognition...")

	// Get image list from environment variable
	testImages := getImageList()

	for _, filename := range testImages {
		testFaceRecognition(ctx, lumenClient, filename, FaceDetectEmbedTask)
	}

	fmt.Println("\nFace recognition tests completed!")
}

// getImageList returns image files from environment variable
func getImageList() []string {
	images := os.Getenv("RECOGNIZE_IMAGES")
	if images == "" {
		fmt.Println("Error: RECOGNIZE_IMAGES environment variable not set")
		fmt.Println("Usage: RECOGNIZE_IMAGES=\"image1.jpg,image2.png,image3.jpeg\" go run main.go")
		os.Exit(1)
	}
	return strings.Split(images, ",")
}

func testFaceRecognition(ctx context.Context, lumenClient *client.LumenClient, filename string, faceDetectEmbedTask string) {
	// Load image file
	imageData, err := os.ReadFile(filename)
	if err != nil {
		fmt.Printf("Failed to load %s: %v\n", filename, err)
		return
	}

	fmt.Printf("Testing %s (%d bytes)\n", filename, len(imageData))

	// Create face recognition request with advanced configuration
	faceReq, err := types.NewFaceRecognitionRequest(imageData,
		types.WithDetectionConfidenceThreshold(0.75), // Higher confidence for recognition
		types.WithMaxFaces(10),                     // Limit to 10 faces for performance
		types.WithFaceSizeMin(50.0),                 // Minimum face size for quality
	)
	if err != nil {
		fmt.Printf("Failed to create face recognition request: %v\n", err)
		return
	}

	inferReq := types.NewInferRequest(faceDetectEmbedTask).
		WithCorrelationID("face_recognize_test").
		ForFaceDetection(faceReq, faceDetectEmbedTask).
		Build()

	// Perform face detection and embedding with retry
	resp, err := lumenClient.InferWithRetry(ctx, inferReq,
		client.WithMaxWaitTime(45*time.Second), // Longer timeout for recognition
		client.WithRetryInterval(3*time.Second),
		client.WithWaitForTask(true))

	if err != nil {
		fmt.Printf("Face recognition failed: %v\n", err)
		return
	}

	// Parse and display results
	faceResp, err := types.ParseInferResponse(resp).AsFaceResponse()
	rawResp := types.ParseInferResponse(resp).Raw()
	if err != nil {
		fmt.Printf("Failed to parse response: %v\n, raw response: %s", err, rawResp)
		return
	}

	fmt.Printf("Success! Detected %d faces with embeddings:\n", faceResp.Count)
	fmt.Printf("   Model: %s\n", faceResp.ModelID)

	for i, face := range faceResp.Faces {
		fmt.Printf("   Face %d: confidence=%.2f, bbox=[%.1f,%.1f,%.1f,%.1f]\n",
			i+1, face.Confidence,
			face.BBox[0], face.BBox[1], face.BBox[2], face.BBox[3])

		// Show landmarks if available
		if len(face.Landmarks) > 0 {
			fmt.Printf("             landmarks: %d points\n", len(face.Landmarks)/2)
		}

		// Show embedding info if available (key for recognition)
		if len(face.Embedding) > 0 {
			fmt.Printf("             embedding: %d dimensions\n", len(face.Embedding))
			// Show first few embedding values for verification
			fmt.Printf("             first 5 values: [%.4f, %.4f, %.4f, %.4f, %.4f]\n",
				face.Embedding[0], face.Embedding[1], face.Embedding[2],
				face.Embedding[3], face.Embedding[4])
		} else {
			fmt.Printf("             embedding: not available\n")
		}
	}

	// If multiple faces, show face similarity comparison
	if len(faceResp.Faces) > 1 && len(faceResp.Faces[0].Embedding) > 0 && len(faceResp.Faces[1].Embedding) > 0 {
		fmt.Printf("   Face similarity comparison:\n")
		for i := 0; i < len(faceResp.Faces)-1; i++ {
			for j := i + 1; j < len(faceResp.Faces); j++ {
				if len(faceResp.Faces[i].Embedding) > 0 && len(faceResp.Faces[j].Embedding) > 0 {
					// Create temporary EmbeddingV1 objects for comparison
					emb1 := types.EmbeddingV1{
						Vector:  faceResp.Faces[i].Embedding,
						ModelID: faceResp.ModelID,
					}
					emb2 := types.EmbeddingV1{
						Vector:  faceResp.Faces[j].Embedding,
						ModelID: faceResp.ModelID,
					}

					similarity, _ := emb1.CosineSimilarity(emb2)
					fmt.Printf("             Face %d vs Face %d: similarity=%.3f\n",
						i+1, j+1, similarity)
				}
			}
		}
	}
}