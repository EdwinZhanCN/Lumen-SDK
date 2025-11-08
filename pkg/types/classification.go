package types

import (
	"fmt"
	"sort"

	"github.com/gabriel-vasile/mimetype"
)

// LabelsV1 represents image classification results with confidence scores.
//
// Classification results consist of multiple labels (categories) with associated
// confidence scores indicating the likelihood that each label applies to the input.
// Labels are typically pre-sorted by confidence score in descending order.
//
// Role in project: Output structure for image classification tasks. Used extensively
// in content categorization, object detection, scene recognition, and automated tagging.
//
// Example:
//
//	result, _ := client.Infer(ctx, classificationRequest)
//	classification, _ := types.ParseInferResponse(result).AsClassificationResponse()
//	fmt.Printf("Model: %s\n", classification.ModelID)
//	for _, label := range classification.TopK(5) {
//	    fmt.Printf("%s: %.2f%%\n", label.Label, label.Score*100)
//	}
type LabelsV1 struct {
	Labels  []Label `json:"labels" example:"[{\"label\": \"cat\", \"score\": 0.9}, {\"label\": \"dog\", \"score\": 0.1}]"`
	ModelID string  `json:"model_id" example:"embedding_model_1"`
}

// Label represents a single classification category with its confidence score.
//
// Role in project: Individual classification result pairing a label name with
// its confidence score. Used for ranking and filtering classification results.
type Label struct {
	Label string  `json:"label"`
	Score float32 `json:"score"`
}

// TopK returns the top K most confident labels from the classification results.
//
// This method sorts labels by confidence score (if not already sorted) and returns
// the K highest scoring labels. If K exceeds the number of labels, all labels are returned.
//
// Parameters:
//   - k: Number of top labels to return
//
// Returns:
//   - []Label: Slice of top K labels sorted by confidence (descending)
//
// Role in project: Enables easy extraction of most relevant classification results,
// commonly used for displaying top predictions to users or filtering low-confidence results.
//
// Example:
//
//	classification, _ := types.ParseInferResponse(result).AsClassificationResponse()
//	topLabels := classification.TopK(3)
//	fmt.Println("Top 3 predictions:")
//	for i, label := range topLabels {
//	    fmt.Printf("%d. %s (%.1f%% confidence)\n",
//	        i+1, label.Label, label.Score*100)
//	}
func (l LabelsV1) TopK(k int) []Label {
	if k > len(l.Labels) {
		k = len(l.Labels)
	}
	sort.Slice(l.Labels, func(i, j int) bool {
		return l.Labels[i].Score > l.Labels[j].Score
	})
	return l.Labels[:k]
}

// ClassificationRequest represents a request for image classification.
//
// This structure encapsulates the image payload and its MIME type for classification.
// Only image types are supported (see SupportedImageMimeTypes). Use with the
// InferRequest builder's ForClassification() method.
//
// Role in project: Input data structure for image classification operations. Works
// with NewClassificationRequest() for automatic MIME type detection and validation.
//
// Example:
//
//	imageData, _ := os.ReadFile("photo.jpg")
//	classReq, err := types.NewClassificationRequest(imageData)
//	if err != nil {
//	    log.Fatal(err)
//	}
type ClassificationRequest struct {
	Payload     []byte `json:"payload"`
	PayloadMime string `json:"payload_mime_type"`
}

// NewClassificationRequest creates a new ClassificationRequest with automatic MIME detection.
//
// This function analyzes the payload to detect its image format and validates that it's
// a supported type for classification. Supported formats include JPEG, PNG, GIF, BMP,
// WebP, and other common image formats (see SupportedImageMimeTypes).
//
// Parameters:
//   - payload: The raw image bytes to classify
//
// Returns:
//   - *ClassificationRequest: Request object ready for ForClassification()
//   - error: Non-nil if the payload is not a supported image type
//
// Role in project: Factory function that simplifies classification request creation with
// automatic format detection. Prevents errors from incorrect MIME type specification.
//
// Example:
//
//	// Classify an image file
//	imageData, err := os.ReadFile("nature_scene.jpg")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	classReq, err := types.NewClassificationRequest(imageData)
//	if err != nil {
//	    log.Fatalf("Unsupported image format: %v", err)
//	}
//
//	inferReq := types.NewInferRequest("scene_classification").
//	    ForClassification(classReq, "scene_classification").
//	    Build()
//
//	result, _ := client.Infer(ctx, inferReq)
//	labels, _ := types.ParseInferResponse(result).AsClassificationResponse()
//	fmt.Printf("Scene: %s\n", labels.TopK(1)[0].Label)
func NewClassificationRequest(payload []byte) (*ClassificationRequest, error) {
	mime := mimetype.Detect(payload)
	mimeString := mime.String()

	// Check if detected MIME type matches any supported image type
	if mimetype.EqualsAny(mimeString, SupportedImageMimeTypes...) {
		return &ClassificationRequest{
			Payload:     payload,
			PayloadMime: mimeString,
		}, nil
	}

	return nil, fmt.Errorf("unsupported payload type: %s", mimeString)
}
