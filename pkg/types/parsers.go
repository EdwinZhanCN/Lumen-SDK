package types

import (
	"encoding/json"
	"fmt"

	pb "github.com/edwinzhancn/lumen-sdk/proto"
)

// InferResponseParser provides type-safe parsing of ML inference responses.
//
// This parser handles the deserialization and type conversion of protobuf responses
// into Go structs. It validates response MIME types and ensures the correct schema
// is used for each response type (embedding, classification, face detection).
//
// Role in project: Bridges the gap between protobuf responses and Go application code.
// Provides type safety and validation to prevent runtime errors from mismatched response types.
//
// Example:
//
//	result, _ := client.Infer(ctx, request)
//	parser := types.ParseInferResponse(result)
//	embeddingResp, err := parser.AsEmbeddingResponse()
//	if err != nil {
//	    log.Fatalf("Failed to parse response: %v", err)
//	}
type InferResponseParser struct {
	resp *pb.InferResponse
}

// ParseInferResponse creates a new parser for the given inference response.
//
// This is the entry point for response parsing. After creating a parser, use the
// appropriate As* method to convert to the expected response type.
//
// Parameters:
//   - resp: The protobuf inference response from the ML node
//
// Returns:
//   - *InferResponseParser: A parser instance ready for type conversion
//
// Role in project: Factory function that initiates the response parsing chain.
// This is typically called immediately after receiving a response from Infer() or InferStream().
//
// Example:
//
//	result, err := client.Infer(ctx, inferReq)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	parser := types.ParseInferResponse(result)
func ParseInferResponse(resp *pb.InferResponse) *InferResponseParser {
	return &InferResponseParser{resp: resp}
}

// AsFaceResponse parses the response as face detection/recognition results.
//
// This method validates that the response has the correct MIME type (application/json;schema=face_v1)
// and deserializes it into a FaceV1 structure containing detected faces with their
// bounding boxes, confidence scores, landmarks, and optional embeddings.
//
// Returns:
//   - *FaceV1: Parsed face detection results with all detected faces
//   - error: Non-nil if MIME type is incorrect or JSON parsing fails
//
// Role in project: Type-safe conversion for face detection responses. Essential for
// applications performing face detection, recognition, or biometric operations.
//
// Example:
//
//	// Detect faces in an image
//	imageData, _ := os.ReadFile("photo.jpg")
//	faceReq, _ := types.NewFaceRecognitionRequest(imageData,
//	    types.WithMaxFaces(5))
//	inferReq := types.NewInferRequest("face_detection").
//	    ForFaceDetection(faceReq, "face_detection").
//	    Build()
//
//	result, _ := client.Infer(ctx, inferReq)
//	faceResp, err := types.ParseInferResponse(result).AsFaceResponse()
//	if err != nil {
//	    log.Fatalf("Failed to parse face response: %v", err)
//	}
//
//	fmt.Printf("Found %d faces\n", faceResp.Count)
//	for i, face := range faceResp.Faces {
//	    fmt.Printf("Face %d: confidence=%.2f\n", i+1, face.Confidence)
//	}
func (p *InferResponseParser) AsFaceResponse() (*FaceV1, error) {
	if p.resp.ResultMime != "application/json;schema=face_v1" {
		return nil, fmt.Errorf("unexpected response type: %s", p.resp.ResultMime)
	}

	var result FaceV1
	if err := json.Unmarshal(p.resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse detection response: %w", err)
	}
	return &result, nil
}

// AsEmbeddingResponse parses the response as an embedding vector result.
//
// This method validates the MIME type (application/json;schema=embedding_v1) and converts
// the response into an EmbeddingV1 structure containing a float32 vector, dimension info,
// and model identifier. The resulting embedding can be used for similarity calculations.
//
// Returns:
//   - *EmbeddingV1: Parsed embedding with vector, dimension, and model ID
//   - error: Non-nil if MIME type is incorrect or JSON parsing fails
//
// Role in project: Type-safe conversion for embedding responses. Embeddings are the most
// common output type in the Lumen SDK, used for semantic search, recommendation systems,
// and similarity comparisons.
//
// Example:
//
//	// Generate text embedding
//	text := []byte("Machine learning transforms data into insights")
//	embReq, _ := types.NewEmbeddingRequest(text)
//	inferReq := types.NewInferRequest("text_embedding").
//	    ForEmbedding(embReq, "text_embedding").
//	    Build()
//
//	result, _ := client.Infer(ctx, inferReq)
//	embedding, err := types.ParseInferResponse(result).AsEmbeddingResponse()
//	if err != nil {
//	    log.Fatalf("Failed to parse embedding: %v", err)
//	}
//
//	fmt.Printf("Embedding dimension: %d\n", embedding.DimValue())
//	fmt.Printf("Model: %s\n", embedding.ModelID)
//
//	// Compare with another embedding
//	similarity, _ := embedding.CosineSimilarity(otherEmbedding)
//	fmt.Printf("Cosine similarity: %.4f\n", similarity)
func (p *InferResponseParser) AsEmbeddingResponse() (*EmbeddingV1, error) {
	if p.resp.ResultMime != "application/json;schema=embedding_v1" {
		return nil, fmt.Errorf("unexpected response type: %s", p.resp.ResultMime)
	}

	var result EmbeddingV1
	if err := json.Unmarshal(p.resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse embedding response: %w", err)
	}
	return &result, nil
}

// AsClassificationResponse parses the response as image classification results.
//
// This method validates the MIME type (application/json;schema=labels_v1) and deserializes
// the response into a LabelsV1 structure containing classification labels with confidence
// scores. The labels are typically sorted by confidence, and you can use TopK() to get
// the most likely categories.
//
// Returns:
//   - *LabelsV1: Parsed classification with labels, scores, and model ID
//   - error: Non-nil if MIME type is incorrect or JSON parsing fails
//
// Role in project: Type-safe conversion for classification responses. Used extensively
// in content categorization, object detection, scene recognition, and automated tagging systems.
//
// Example:
//
//	// Classify an image
//	imageData, _ := os.ReadFile("nature.jpg")
//	classReq, _ := types.NewClassificationRequest(imageData)
//	inferReq := types.NewInferRequest("image_classification").
//	    ForClassification(classReq, "scene_classification").
//	    Build()
//
//	result, _ := client.Infer(ctx, inferReq)
//	classification, err := types.ParseInferResponse(result).AsClassificationResponse()
//	if err != nil {
//	    log.Fatalf("Failed to parse classification: %v", err)
//	}
//
//	// Get top 3 predictions
//	topLabels := classification.TopK(3)
//	fmt.Println("Top predictions:")
//	for i, label := range topLabels {
//	    fmt.Printf("%d. %s (%.2f%%)\n", i+1, label.Label, label.Score*100)
//	}
func (p *InferResponseParser) AsClassificationResponse() (*LabelsV1, error) {
	if p.resp.ResultMime != "application/json;schema=labels_v1" {
		return nil, fmt.Errorf("unexpected response type: %s", p.resp.ResultMime)
	}

	var result LabelsV1
	if err := json.Unmarshal(p.resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse classification response: %w", err)
	}
	return &result, nil
}

// AsOCRResponse parses the response as OCR results.
//
// This method validates that the response has the correct MIME type (application/json;schema=ocr_v1)
// and deserializes it into an OCRV1 structure containing detected text regions.
//
// Returns:
//   - *OCRV1: Parsed OCR results with all detected text items
//   - error: Non-nil if MIME type is incorrect or JSON parsing fails
//
// Role in project: Type-safe conversion for OCR responses.
func (p *InferResponseParser) AsOCRResponse() (*OCRV1, error) {
	if p.resp.ResultMime != "application/json;schema=ocr_v1" {
		return nil, fmt.Errorf("unexpected response type: %s", p.resp.ResultMime)
	}

	var result OCRV1
	if err := json.Unmarshal(p.resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse OCR response: %w", err)
	}
	return &result, nil
}

// Raw returns the underlying protobuf response without parsing.
//
// Use this method when you need direct access to the raw response fields,
// such as custom metadata, correlation IDs, or when implementing custom
// response handling logic.
//
// Returns:
//   - *pb.InferResponse: The original protobuf response
//
// Role in project: Provides escape hatch for advanced use cases that need
// access to raw response data not covered by the typed parsers.
//
// Example:
//
//	result, _ := client.Infer(ctx, inferReq)
//	parser := types.ParseInferResponse(result)
//	raw := parser.Raw()
//	fmt.Printf("Correlation ID: %s\n", raw.CorrelationId)
//	fmt.Printf("Is final: %v\n", raw.IsFinal)
func (p *InferResponseParser) Raw() *pb.InferResponse {
	return p.resp
}
