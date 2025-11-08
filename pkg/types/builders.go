package types

import (
	"fmt"

	pb "github.com/edwinzhancn/lumen-sdk/proto"
)

// InferRequestBuilder provides a fluent interface for constructing inference requests.
//
// This builder pattern implementation allows for clean, readable request construction
// with method chaining. It handles the complexity of setting up various request types
// (embedding, classification, face detection) with appropriate metadata and configurations.
//
// Role in project: Simplifies the creation of well-formed inference requests for end users.
// The builder pattern prevents common mistakes and ensures all required fields are set correctly.
//
// Example:
//
//	req := types.NewInferRequest("text_embedding").
//	    WithCorrelationID("my-request-123").
//	    WithMeta("model_version", "v2").
//	    ForEmbedding(embeddingReq, "text_embedding").
//	    Build()
type InferRequestBuilder struct {
	req *pb.InferRequest
}

// NewInferRequest creates a new InferRequestBuilder for the specified task.
//
// The task name is used by the client to select appropriate ML nodes from the catalog
// based on their capabilities. Each node advertises the tasks it supports, and the
// client's load balancer uses this information for routing.
//
// Parameters:
//   - task: The ML task identifier (e.g., "text_embedding", "face_detection", "classification")
//
// Returns:
//   - *InferRequestBuilder: A new builder instance ready for method chaining
//
// Role in project: Entry point for creating type-safe inference requests. This is typically
// the first method called when preparing an ML inference operation.
//
// Example:
//
//	// Create a builder for text embedding task
//	builder := types.NewInferRequest("text_embedding")
//
//	// Or with immediate chaining
//	req := types.NewInferRequest("face_detection").
//	    WithCorrelationID("detection-001").
//	    Build()
func NewInferRequest(task string) *InferRequestBuilder {
	return &InferRequestBuilder{
		req: &pb.InferRequest{
			Task:        task,
			Meta:        make(map[string]string),
			PayloadMime: "application/json",
		},
	}
}

// WithCorrelationID sets a custom correlation ID for request tracking and logging.
//
// Correlation IDs enable request tracing across distributed components and make debugging
// easier. If not provided, the system automatically generates a unique correlation ID.
// Using custom IDs is recommended for production systems with distributed tracing.
//
// Parameters:
//   - id: A unique identifier for this request (e.g., UUID, trace ID)
//
// Returns:
//   - *InferRequestBuilder: The builder instance for method chaining
//
// Role in project: Enables distributed tracing and log correlation across the entire
// inference pipeline (client -> load balancer -> ML node -> response).
//
// Example:
//
//	req := types.NewInferRequest("embedding").
//	    WithCorrelationID(uuid.New().String()).
//	    Build()
func (b *InferRequestBuilder) WithCorrelationID(id string) *InferRequestBuilder {
	b.req.CorrelationId = id
	return b
}

// WithMeta adds a metadata key-value pair to the inference request.
//
// Metadata provides additional configuration and context for ML inference operations.
// Different tasks support different metadata fields. Common uses include model selection,
// confidence thresholds, and output format preferences.
//
// Task-specific metadata examples:
//   - Face detection: "detection_confidence_threshold", "max_faces", "nms_threshold"
//   - Embedding: "model_version", "normalize", "output_format"
//   - Classification: "top_k", "confidence_threshold"
//
// Parameters:
//   - key: Metadata field name (task-specific, see ML node documentation)
//   - value: Metadata field value as a string
//
// Returns:
//   - *InferRequestBuilder: The builder instance for method chaining
//
// Role in project: Provides flexible configuration mechanism for ML tasks without
// requiring API changes when new parameters are added to ML models.
//
// Example:
//
//	req := types.NewInferRequest("face_detection").
//	    WithMeta("detection_confidence_threshold", "0.8").
//	    WithMeta("max_faces", "10").
//	    Build()
func (b *InferRequestBuilder) WithMeta(key, value string) *InferRequestBuilder {
	if b.req.Meta == nil {
		b.req.Meta = make(map[string]string)
	}
	b.req.Meta[key] = value
	return b
}

// Build finalizes and returns the constructed inference request.
//
// This method completes the builder chain and produces the protobuf InferRequest
// that can be sent to the LumenClient for processing. After calling Build(),
// the builder should not be reused.
//
// Returns:
//   - *pb.InferRequest: The fully constructed inference request ready for submission
//
// Role in project: Final step in the builder pattern that produces the actual request
// object consumed by the client's Infer() or InferStream() methods.
//
// Example:
//
//	req := types.NewInferRequest("classification").
//	    WithCorrelationID("img-001").
//	    ForClassification(classReq, "image_classification").
//	    Build()
//	result, err := client.Infer(ctx, req)
func (b *InferRequestBuilder) Build() *pb.InferRequest {
	return b.req
}

// ForEmbedding configures the builder for an embedding generation request.
//
// Embedding requests transform text or images into high-dimensional vector representations
// useful for semantic search, similarity comparisons, and clustering. This method sets
// the appropriate payload and MIME type for the embedding operation.
//
// Parameters:
//   - req: The embedding request containing the payload (use NewEmbeddingRequest for automatic MIME detection)
//   - task: The specific embedding task name from node capabilities (e.g., "lumen_clip_embed", "text_embedding")
//
// Returns:
//   - *InferRequestBuilder: The builder instance for method chaining
//
// Role in project: Specialized builder method for embedding tasks, the most commonly used
// ML operation in the Lumen SDK. Embeddings power features like semantic search, image
// similarity, and content recommendation.
//
// Example:
//
//	// Text embedding
//	textData := []byte("Machine learning is fascinating")
//	embReq, _ := types.NewEmbeddingRequest(textData)
//	req := types.NewInferRequest("text_embedding").
//	    ForEmbedding(embReq, "lumen_clip_embed").
//	    Build()
//
//	// Image embedding
//	imageData, _ := os.ReadFile("photo.jpg")
//	embReq, _ := types.NewEmbeddingRequest(imageData)
//	req := types.NewInferRequest("image_embedding").
//	    ForEmbedding(embReq, "lumen_clip_image_embed").
//	    Build()
func (b *InferRequestBuilder) ForEmbedding(req *EmbeddingRequest, task string) *InferRequestBuilder {
	payload := req.Payload
	b.req.Payload = payload
	b.req.PayloadMime = req.PayloadMime
	b.req.Task = task
	return b
}

// ForClassification configures the builder for an image classification request.
//
// Classification requests categorize images into predefined classes with confidence scores.
// This is commonly used for content moderation, scene detection, object recognition, and
// automated tagging systems.
//
// Parameters:
//   - req: The classification request with image payload (use NewClassificationRequest for MIME detection)
//   - task: The classification task name from node capabilities (e.g., "lumen_clip_classify", "scene_classification")
//
// Returns:
//   - *InferRequestBuilder: The builder instance for method chaining
//
// Role in project: Specialized builder for classification tasks. Classification is widely
// used for categorizing visual content, detecting objects, identifying scenes, and content
// filtering applications.
//
// Example:
//
//	// Basic image classification
//	imageData, _ := os.ReadFile("photo.jpg")
//	classReq, _ := types.NewClassificationRequest(imageData)
//	req := types.NewInferRequest("image_classification").
//	    ForClassification(classReq, "lumen_clip_classify").
//	    Build()
//
//	result, err := client.Infer(ctx, req)
//	classResp, _ := types.ParseInferResponse(result).AsClassificationResponse()
//	topLabels := classResp.TopK(5)
//	for _, label := range topLabels {
//	    fmt.Printf("%s: %.2f\n", label.Label, label.Score)
//	}
func (b *InferRequestBuilder) ForClassification(req *ClassificationRequest, task string) *InferRequestBuilder {
	payload := req.Payload
	b.req.Payload = payload
	b.req.PayloadMime = req.PayloadMime
	b.req.Task = task
	return b
}

// ForFaceDetection configures the builder for face detection and recognition requests.
//
// Face detection requests locate and analyze faces in images, optionally returning
// facial landmarks, bounding boxes, and face embeddings for recognition. This method
// automatically sets task-specific metadata from the FaceRecognitionRequest configuration.
//
// Supported parameters (set via metadata):
//   - detection_confidence_threshold: Minimum confidence for face detection (0.0-1.0)
//   - nms_threshold: Non-maximum suppression threshold for overlapping detections
//   - face_size_min/max: Constraints on detected face sizes
//   - max_faces: Maximum number of faces to detect (-1 for unlimited)
//
// Parameters:
//   - req: Face detection request with image and optional configuration parameters
//   - task: The face detection task name (e.g., "face_detection", "face_recognition")
//
// Returns:
//   - *InferRequestBuilder: The builder instance for method chaining
//
// Role in project: Specialized builder for face detection/recognition tasks. Used in
// applications like security systems, photo organization, attendance tracking, and
// identity verification.
//
// Example:
//
//	// Face detection with custom thresholds
//	imageData, _ := os.ReadFile("group_photo.jpg")
//	faceReq, _ := types.NewFaceRecognitionRequest(imageData,
//	    types.WithDetectionConfidenceThreshold(0.8),
//	    types.WithMaxFaces(10),
//	)
//	req := types.NewInferRequest("face_detection").
//	    ForFaceDetection(faceReq, "face_detection").
//	    Build()
//
//	result, err := client.Infer(ctx, req)
//	faceResp, _ := types.ParseInferResponse(result).AsFaceResponse()
//	fmt.Printf("Detected %d faces\n", faceResp.Count)
//	for i, face := range faceResp.Faces {
//	    fmt.Printf("Face %d: confidence=%.2f, bbox=%v\n",
//	        i+1, face.Confidence, face.BBox)
//	}
func (b *InferRequestBuilder) ForFaceDetection(req *FaceRecognitionRequest, task string) *InferRequestBuilder {
	payload := req.Payload
	b.req.Payload = payload
	b.req.PayloadMime = req.PayloadMime
	b.req.Task = task

	// Set face detection metadata from request configuration
	if req.DetectionConfidenceThreshold > 0 {
		b.WithMeta("detection_confidence_threshold", fmt.Sprintf("%.3f", req.DetectionConfidenceThreshold))
	}
	if req.NmsThreshold > 0 {
		b.WithMeta("nms_threshold", fmt.Sprintf("%.3f", req.NmsThreshold))
	}
	if req.FaceSizeMin > 0 {
		b.WithMeta("face_size_min", fmt.Sprintf("%.1f", req.FaceSizeMin))
	}
	if req.FaceSizeMax > 0 {
		b.WithMeta("face_size_max", fmt.Sprintf("%.1f", req.FaceSizeMax))
	}

	// Max Faces cannot be 0 or samller than -1
	if req.MaxFaces != 0 && req.MaxFaces >= -1 {
		b.WithMeta("max_faces", fmt.Sprintf("%d", req.MaxFaces))
	}

	return b
}
