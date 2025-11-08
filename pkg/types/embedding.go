package types

import (
	"fmt"
	"math"

	"github.com/gabriel-vasile/mimetype"
)

// EmbeddingV1 represents a high-dimensional vector embedding from ML models.
//
// Embeddings are dense vector representations that capture semantic meaning of text,
// images, or other data types. They enable similarity comparisons, semantic search,
// clustering, and recommendation systems.
//
// The vector values are typically normalized or can be normalized using the Normalize()
// method. Common embedding dimensions range from 128 to 1536 depending on the model.
//
// Role in project: Core data structure for embedding operations, the most fundamental
// ML output in the Lumen SDK. Embeddings power semantic search, image similarity,
// recommendation engines, and clustering applications.
//
// Example:
//
//	// Generate and use an embedding
//	result, _ := client.Infer(ctx, embeddingRequest)
//	embedding, _ := types.ParseInferResponse(result).AsEmbeddingResponse()
//
//	fmt.Printf("Dimensions: %d\n", embedding.DimValue())
//	fmt.Printf("Model: %s\n", embedding.ModelID)
//	fmt.Printf("Magnitude: %.4f\n", embedding.Magnitude())
//
//	// Compare with another embedding
//	similarity, _ := embedding.CosineSimilarity(otherEmbedding)
//	if similarity > 0.9 {
//	    fmt.Println("Highly similar!")
//	}
type EmbeddingV1 struct {
	Vector  []float32 `json:"vector" example:"[0.1, 0.2, 0.3]"`
	Dim     int       `json:"dim" example:"3"`
	ModelID string    `json:"model_id" example:"embedding_model_1"`
}

// DimValue returns the actual dimension of the embedding vector.
//
// This is computed from the vector length and may differ from the Dim field
// if the model output was truncated or padded. Always use DimValue() for
// accurate dimension information.
//
// Returns:
//   - int: The number of dimensions in the vector
//
// Example:
//
//	embedding, _ := types.ParseInferResponse(result).AsEmbeddingResponse()
//	dim := embedding.DimValue()
//	fmt.Printf("Vector has %d dimensions\n", dim)
func (e EmbeddingV1) DimValue() int {
	return len(e.Vector)
}

// IsEmpty Returns true if the embedding is empty
func (e EmbeddingV1) IsEmpty() bool {
	return len(e.Vector) == 0
}

// Normalize performs L2 normalization on the embedding vector.
//
// L2 normalization scales the vector to unit length (magnitude = 1.0), which is
// essential for computing cosine similarity and ensures consistent distance metrics.
// Many embedding models output pre-normalized vectors, but this method can be used
// to ensure normalization or to re-normalize after vector arithmetic.
//
// Returns:
//   - EmbeddingV1: A new embedding with normalized vector (original is unchanged)
//
// Role in project: Prepares embeddings for similarity calculations. Normalized vectors
// allow cosine similarity to be computed using just dot product, which is much faster.
//
// Example:
//
//	embedding, _ := types.ParseInferResponse(result).AsEmbeddingResponse()
//	normalized := embedding.Normalize()
//	fmt.Printf("Original magnitude: %.4f\n", embedding.Magnitude())
//	fmt.Printf("Normalized magnitude: %.4f\n", normalized.Magnitude()) // Should be ~1.0
func (e EmbeddingV1) Normalize() EmbeddingV1 {
	if e.IsEmpty() {
		return e
	}

	vec := e.Vector

	norm := float32(0.0)
	for _, v := range vec {
		norm += v * v
	}
	norm = float32(math.Sqrt(float64(norm)))

	if norm == 0.0 {
		return e
	}

	normalized := make([]float32, len(vec))
	for i, v := range vec {
		normalized[i] = v / norm
	}

	return EmbeddingV1{
		Vector:  normalized,
		Dim:     len(normalized),
		ModelID: e.ModelID,
	}
}

func (e EmbeddingV1) Magnitude() float32 {
	if e.IsEmpty() {
		return 0.0
	}

	sum := float32(0.0)
	for _, v := range e.Vector {
		sum += v * v
	}

	return float32(math.Sqrt(float64(sum)))
}

// Dot Computes the dot product of two embeddings
func (e EmbeddingV1) Dot(other EmbeddingV1) (float32, error) {
	if len(e.Vector) != len(other.Vector) {
		return 0.0, fmt.Errorf("vector dimensions mismatch: %d vs %d", len(e.Vector), len(other.Vector))
	}

	if e.IsEmpty() {
		return 0.0, nil
	}

	sum := float32(0.0)
	for i, v := range e.Vector {
		sum += v * other.Vector[i]
	}

	return sum, nil
}

// CosineSimilarity computes the cosine similarity between two embeddings.
//
// Cosine similarity measures the cosine of the angle between two vectors, ranging
// from -1 (opposite) to +1 (identical), with 0 indicating orthogonality. This is
// the standard similarity metric for embeddings and is invariant to vector magnitude.
//
// The vectors must have the same dimensions. For best results, use normalized embeddings.
//
// Parameters:
//   - other: The embedding to compare against
//
// Returns:
//   - float32: Similarity score in range [-1, 1], where higher is more similar
//   - error: Non-nil if dimensions don't match
//
// Role in project: Primary method for comparing embeddings in semantic search, similarity
// ranking, duplicate detection, and recommendation systems. This is the most commonly
// used distance metric in the Lumen SDK.
//
// Example:
//
//	// Compare two text embeddings
//	text1 := []byte("machine learning")
//	text2 := []byte("artificial intelligence")
//	emb1, _ := generateEmbedding(text1)
//	emb2, _ := generateEmbedding(text2)
//
//	similarity, err := emb1.CosineSimilarity(emb2)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	fmt.Printf("Similarity: %.4f\n", similarity)
//	if similarity > 0.8 {
//	    fmt.Println("Highly similar concepts!")
//	}
func (e EmbeddingV1) CosineSimilarity(other EmbeddingV1) (float32, error) {
	if len(e.Vector) != len(other.Vector) {
		return 0.0, fmt.Errorf("vector dimensions mismatch: %d vs %d", len(e.Vector), len(other.Vector))
	}

	if e.IsEmpty() || other.IsEmpty() {
		return 0.0, nil
	}

	// 计算点积
	dot, err := e.Dot(other)
	if err != nil {
		return 0.0, err
	}

	// 计算模长
	mag1 := e.Magnitude()
	mag2 := other.Magnitude()

	if mag1 == 0.0 || mag2 == 0.0 {
		return 0.0, nil
	}

	return dot / (mag1 * mag2), nil
}

// EuclideanDistance Computes the Euclidean distance between two embeddings
func (e EmbeddingV1) EuclideanDistance(other EmbeddingV1) (float32, error) {
	if len(e.Vector) != len(other.Vector) {
		return 0.0, fmt.Errorf("vector dimensions mismatch: %d vs %d", len(e.Vector), len(other.Vector))
	}

	if e.IsEmpty() {
		return 0.0, nil
	}

	sum := float32(0.0)
	for i, v := range e.Vector {
		diff := v - other.Vector[i]
		sum += diff * diff
	}

	return float32(math.Sqrt(float64(sum))), nil
}

// ManhattanDistance Computes the Manhattan distance between two embeddings
func (e EmbeddingV1) ManhattanDistance(other EmbeddingV1) (float32, error) {
	if len(e.Vector) != len(other.Vector) {
		return 0.0, fmt.Errorf("vector dimensions mismatch: %d vs %d", len(e.Vector), len(other.Vector))
	}

	if e.IsEmpty() {
		return 0.0, nil
	}

	sum := float32(0.0)
	for i, v := range e.Vector {
		diff := v - other.Vector[i]
		if diff < 0 {
			sum -= diff
		} else {
			sum += diff
		}
	}

	return sum, nil
}

// EmbeddingRequest represents a request for embedding generation.
//
// This structure encapsulates the payload (text or image bytes) and its MIME type
// for embedding generation. Additional parameters like ModelID, CorrelationID, and
// metadata are set through the InferRequest builder.
//
// Role in project: Input data structure for embedding operations. Works with
// NewEmbeddingRequest() for automatic MIME type detection and validation.
//
// Example:
//
//	// Create embedding request with auto-detected MIME type
//	textData := []byte("semantic search query")
//	embReq, err := types.NewEmbeddingRequest(textData)
//	if err != nil {
//	    log.Fatal(err)
//	}
type EmbeddingRequest struct {
	Payload     []byte `json:"payload"`
	PayloadMime string `json:"payload_mime_type"`
}

// NewEmbeddingRequest creates a new EmbeddingRequest with automatic MIME type detection.
//
// This function analyzes the payload to determine if it's text or image data and
// validates that the MIME type is supported for embedding generation. Supported types
// include text/plain, text/html, image/jpeg, image/png, and others.
//
// Parameters:
//   - payload: The raw bytes of text or image data to embed
//
// Returns:
//   - *EmbeddingRequest: Request object ready to use with ForEmbedding()
//   - error: Non-nil if MIME type is unsupported
//
// Role in project: Factory function that simplifies embedding request creation by
// automatically detecting and validating content types. Prevents common errors from
// incorrect MIME type specification.
//
// Example:
//
//	// Text embedding
//	text := []byte("Natural language processing")
//	embReq, err := types.NewEmbeddingRequest(text)
//	if err != nil {
//	    log.Fatalf("Unsupported content type: %v", err)
//	}
//
//	// Image embedding
//	imageData, _ := os.ReadFile("photo.jpg")
//	embReq, err := types.NewEmbeddingRequest(imageData)
//	if err != nil {
//	    log.Fatalf("Unsupported image type: %v", err)
//	}
func NewEmbeddingRequest(payload []byte) (*EmbeddingRequest, error) {
	mime := mimetype.Detect(payload)
	mimeString := mime.String()

	// Check if detected MIME type matches any supported image type
	if mimetype.EqualsAny(mimeString, SupportedImageMimeTypes...) {
		return &EmbeddingRequest{
			Payload:     payload,
			PayloadMime: mimeString,
		}, nil
	}

	// Check if detected MIME type matches any supported text type
	if mimetype.EqualsAny(mimeString, SupportedTextMimeTypes...) {
		return &EmbeddingRequest{
			Payload:     payload,
			PayloadMime: mimeString,
		}, nil
	}

	return nil, fmt.Errorf("unsupported payload type: %s", mimeString)
}
