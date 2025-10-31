package types

import (
	"fmt"
	"math"

	"github.com/gabriel-vasile/mimetype"
)

// EmbeddingV1 is the general embedding_v1 json schema for embed result from Lumen ML Services.
type EmbeddingV1 struct {
	Vector  []float32 `json:"vector" example:"[0.1, 0.2, 0.3]"`
	Dim     int       `json:"dim" example:"3"`
	ModelID string    `json:"model_id" example:"embedding_model_1"`
}

// DimValue Returns the actual dimension of the embedding
func (e EmbeddingV1) DimValue() int {
	return len(e.Vector)
}

// IsEmpty Returns true if the embedding is empty
func (e EmbeddingV1) IsEmpty() bool {
	return len(e.Vector) == 0
}

// Normalize Standardizes the embedding vector (L2 normalization)
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

// CosineSimilarity Computes the cosine similarity between two embeddings
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

// EmbeddingRequest Represents a request for embedding generation. Root InferRequest Builder give ModelID, CorrelationID, and Meta.
type EmbeddingRequest struct {
	Payload     []byte `json:"payload"`
	PayloadMime string `json:"payload_mime_type"`
}

// NewEmbeddingRequest Creates a new EmbeddingRequest instance. The payloadMime must be one of SupportedImageMimeTypes or SupportedTextMimeTypes.
func NewEmbeddingRequest(payload []byte) (*EmbeddingRequest, error) {
	mime := mimetype.Detect(payload)
	if mimetype.EqualsAny(mime.String(), SupportedImageMimeTypes) {
		payloadMime := mime.String()
		return &EmbeddingRequest{
			Payload:     payload,
			PayloadMime: payloadMime,
		}, nil
	}

	if mimetype.EqualsAny(mime.String(), SupportedTextMimeTypes) {
		payloadMime := mime.String()
		return &EmbeddingRequest{
			Payload:     payload,
			PayloadMime: payloadMime,
		}, nil
	}

	return nil, fmt.Errorf("unsupported payload type: %s", mime.String())
}
