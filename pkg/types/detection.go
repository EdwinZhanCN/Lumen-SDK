package types

import (
	"fmt"

	"github.com/gabriel-vasile/mimetype"
)

// FaceV1 represents face detection and recognition results from ML models.
//
// This structure contains all detected faces with their locations, confidence scores,
// facial landmarks, and optional embeddings for recognition. The Count field indicates
// the total number of faces detected.
//
// Role in project: Output structure for face detection and recognition tasks. Used in
// security systems, photo organization, attendance tracking, biometric authentication,
// and identity verification applications.
//
// Example:
//
//	result, _ := client.Infer(ctx, faceDetectionRequest)
//	faceResp, _ := types.ParseInferResponse(result).AsFaceResponse()
//	fmt.Printf("Detected %d faces\n", faceResp.Count)
//	fmt.Printf("Model: %s\n", faceResp.ModelID)
//	for i, face := range faceResp.Faces {
//	    fmt.Printf("Face %d: confidence=%.2f, location=%v\n",
//	        i+1, face.Confidence, face.BBox)
//	}
type FaceV1 struct {
	Faces   []Face `json:"faces"`
	Count   int    `json:"count"`
	ModelID string `json:"model_id"`
}

// Face represents a single detected face with its attributes.
//
// Each face includes:
//   - BBox: Bounding box as [x, y, width, height] in image coordinates
//   - Confidence: Detection confidence score (0.0 to 1.0)
//   - Landmarks: Optional facial keypoints (eyes, nose, mouth corners, etc.)
//   - Embedding: Optional face embedding vector for recognition/comparison
//
// Role in project: Individual face detection result containing location, confidence,
// and optional biometric data for recognition tasks.
type Face struct {
	BBox       []float32 `json:"bbox"` //  [x, y, w, h]
	Confidence float32   `json:"confidence"`
	Landmarks  []float32 `json:"landmarks,omitempty"`
	Embedding  []float32 `json:"embedding,omitempty"`
}

// FaceRecognitionRequest represents a request for face detection and recognition.
//
// This structure encapsulates the image payload and various detection parameters:
//   - DetectionConfidenceThreshold: Minimum confidence for accepting detections (0.0-1.0)
//   - NmsThreshold: Non-maximum suppression threshold for overlapping faces
//   - FaceSizeMin/Max: Constraints on face sizes to detect
//   - MaxFaces: Maximum number of faces to return (-1 for unlimited)
//
// Use the WithXxx option functions to set these parameters cleanly.
//
// Role in project: Input structure for face detection tasks with fine-grained control
// over detection parameters. Supports both simple detection and advanced recognition
// with facial embeddings.
//
// Example:
//
//	imageData, _ := os.ReadFile("group_photo.jpg")
//	faceReq, err := types.NewFaceRecognitionRequest(imageData,
//	    types.WithDetectionConfidenceThreshold(0.85),
//	    types.WithMaxFaces(10),
//	    types.WithFaceSizeMin(20.0),
//	)
type FaceRecognitionRequest struct {
	Payload                      []byte  `json:"payload"`
	PayloadMime                  string  `json:"payload_mime_type"`
	DetectionConfidenceThreshold float32 `json:"detection_confidence_threshold,omitempty"`
	NmsThreshold                 float32 `json:"nms_threshold,omitempty"`
	FaceSizeMin                  float32 `json:"face_size_min,omitempty"`
	FaceSizeMax                  float32 `json:"face_size_max,omitempty"`
	MaxFaces                     int     `json:"max_faces,omitempty"` // -1 means no limit
}

// FaceRecognitionOption is a function type for configuring face detection requests.
//
// This option pattern allows clean, readable configuration of detection parameters
// without requiring many constructor variants or builder methods.
//
// Role in project: Provides flexible configuration mechanism for face detection
// requests using the functional options pattern.
type FaceRecognitionOption func(*FaceRecognitionRequest)

// WithDetectionConfidenceThreshold sets the minimum confidence threshold for face detection.
//
// Only faces detected with confidence above this threshold will be included in results.
// Higher values reduce false positives but may miss some faces. Typical range: 0.5-0.95.
//
// Parameters:
//   - threshold: Confidence threshold (0.0 to 1.0)
//
// Returns:
//   - FaceRecognitionOption: Option function for NewFaceRecognitionRequest
//
// Example:
//
//	faceReq, _ := types.NewFaceRecognitionRequest(imageData,
//	    types.WithDetectionConfidenceThreshold(0.9), // Only high-confidence faces
//	)
func WithDetectionConfidenceThreshold(threshold float32) FaceRecognitionOption {
	return func(req *FaceRecognitionRequest) {
		req.DetectionConfidenceThreshold = threshold
	}
}

// WithNmsThreshold 设置 NMS 阈值
func WithNmsThreshold(threshold float32) FaceRecognitionOption {
	return func(req *FaceRecognitionRequest) {
		req.NmsThreshold = threshold
	}
}

// WithFaceSizeMin 设置最小人脸尺寸
func WithFaceSizeMin(size float32) FaceRecognitionOption {
	return func(req *FaceRecognitionRequest) {
		req.FaceSizeMin = size
	}
}

// WithFaceSizeMax 设置最大人脸尺寸
func WithFaceSizeMax(size float32) FaceRecognitionOption {
	return func(req *FaceRecognitionRequest) {
		req.FaceSizeMax = size
	}
}

// WithMaxFaces sets the maximum number of faces to detect in the image.
//
// Limits the number of faces returned in the response. Use -1 for unlimited faces.
// This is useful for performance optimization and when you only need a fixed number
// of the most confident detections.
//
// Parameters:
//   - maxFaces: Maximum faces to return (-1 for unlimited)
//
// Returns:
//   - FaceRecognitionOption: Option function for NewFaceRecognitionRequest
//
// Example:
//
//	// Detect at most 5 faces
//	faceReq, _ := types.NewFaceRecognitionRequest(imageData,
//	    types.WithMaxFaces(5),
//	)
//
//	// Detect all faces
//	faceReq, _ := types.NewFaceRecognitionRequest(imageData,
//	    types.WithMaxFaces(-1),
//	)
func WithMaxFaces(maxFaces int) FaceRecognitionOption {
	return func(req *FaceRecognitionRequest) {
		req.MaxFaces = maxFaces
	}
}

// NewFaceRecognitionRequest creates a new face detection request with optional configuration.
//
// This function analyzes the payload to detect the image format and validates it's
// a supported type. Configuration options can be passed to customize detection behavior
// using the WithXxx option functions.
//
// Parameters:
//   - payload: The raw image bytes to process
//   - opts: Optional configuration functions (WithDetectionConfidenceThreshold, WithMaxFaces, etc.)
//
// Returns:
//   - *FaceRecognitionRequest: Configured request ready for ForFaceDetection()
//   - error: Non-nil if the payload is not a supported image type
//
// Role in project: Factory function for creating face detection requests with clean,
// flexible configuration. Automatically detects image format and validates MIME type.
//
// Example:
//
//	// Basic face detection
//	imageData, _ := os.ReadFile("photo.jpg")
//	faceReq, err := types.NewFaceRecognitionRequest(imageData)
//
//	// Advanced face detection with custom parameters
//	faceReq, err := types.NewFaceRecognitionRequest(imageData,
//	    types.WithDetectionConfidenceThreshold(0.85),
//	    types.WithMaxFaces(10),
//	    types.WithFaceSizeMin(20.0),
//	    types.WithNmsThreshold(0.4),
//	)
//	if err != nil {
//	    log.Fatalf("Invalid image: %v", err)
//	}
//
//	inferReq := types.NewInferRequest("face_detection").
//	    ForFaceDetection(faceReq, "face_detection").
//	    Build()
func NewFaceRecognitionRequest(payload []byte, opts ...FaceRecognitionOption) (*FaceRecognitionRequest, error) {
	mime := mimetype.Detect(payload)
	mimeString := mime.String()

	// Check if detected MIME type matches any supported image type
	if mimetype.EqualsAny(mimeString, SupportedImageMimeTypes...) {
		req := &FaceRecognitionRequest{
			Payload:     payload,
			PayloadMime: mimeString,
		}

		// 应用所有选项
		for _, opt := range opts {
			opt(req)
		}

		return req, nil
	}

	return nil, fmt.Errorf("unsupported payload type: %s", mimeString)
}
