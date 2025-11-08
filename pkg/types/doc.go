// Package types provides type-safe data structures for ML inference operations.
//
// The types package defines all request/response types, builders, and parsers
// for ML operations in the Lumen SDK. It provides:
//
//   - Request builders for clean, fluent API construction
//   - Response parsers for type-safe result handling
//   - Data structures for embeddings, classifications, and face detection
//   - MIME type constants for supported formats
//
// # Request Builders
//
// Use builders to construct type-safe inference requests:
//
//	inferReq := types.NewInferRequest("text_embedding").
//	    WithCorrelationID("req-123").
//	    WithMeta("model", "v2").
//	    ForEmbedding(embeddingReq, "text_embedding").
//	    Build()
//
// # Response Parsers
//
// Parse responses into strongly-typed structures:
//
//	result, _ := client.Infer(ctx, inferReq)
//	embedding, err := types.ParseInferResponse(result).
//	    AsEmbeddingResponse()
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// # Embedding Operations
//
// Work with vector embeddings for semantic search:
//
//	// Generate embedding
//	text := []byte("semantic search query")
//	embReq, _ := types.NewEmbeddingRequest(text)
//
//	// Compare embeddings
//	similarity, _ := emb1.CosineSimilarity(emb2)
//	if similarity > 0.9 {
//	    fmt.Println("Highly similar!")
//	}
//
// # Classification
//
// Classify images into categories:
//
//	imageData, _ := os.ReadFile("photo.jpg")
//	classReq, _ := types.NewClassificationRequest(imageData)
//	inferReq := types.NewInferRequest("classification").
//	    ForClassification(classReq, "image_classification").
//	    Build()
//
//	result, _ := client.Infer(ctx, inferReq)
//	labels, _ := types.ParseInferResponse(result).
//	    AsClassificationResponse()
//	topLabels := labels.TopK(5)
//
// # Face Detection
//
// Detect and recognize faces in images:
//
//	imageData, _ := os.ReadFile("photo.jpg")
//	faceReq, _ := types.NewFaceRecognitionRequest(imageData,
//	    types.WithDetectionConfidenceThreshold(0.85),
//	    types.WithMaxFaces(10),
//	)
//	inferReq := types.NewInferRequest("face_detection").
//	    ForFaceDetection(faceReq, "face_detection").
//	    Build()
//
// # Role in Project
//
// The types package provides the data layer for ML operations, ensuring
// type safety and clean APIs. It bridges between application code and
// the protobuf-based gRPC communication layer.
package types
