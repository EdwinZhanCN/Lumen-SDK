package types

import (
	"fmt"

	"github.com/gabriel-vasile/mimetype"
)

// OCRV1 represents optical character recognition (OCR) results.
//
// This structure contains all detected text regions with their content, locations,
// and confidence scores. The Count field indicates the total number of text regions detected.
//
// Role in project: Output structure for OCR tasks. Used in document digitization,
// text extraction, license plate recognition, and scene text understanding.
type OCRV1 struct {
	Items   []OCRItem `json:"items"`
	Count   int       `json:"count"`
	ModelID string    `json:"model_id"`
}

// OCRItem represents a single detected text region with its content.
//
// Each item includes:
//   - Box: Polygon coordinates defining the text region (usually 4 points: TL, TR, BR, BL).
//     Each point is [x, y].
//   - Text: Recognized text content.
//   - Confidence: Recognition confidence score (0.0 to 1.0).
type OCRItem struct {
	Box        [][]int `json:"box"` // List of [x, y] points
	Text       string  `json:"text"`
	Confidence float32 `json:"confidence"`
}

// OCRRequest represents a request for optical character recognition.
//
// This structure encapsulates the image payload for text detection and recognition.
// It automatically handles MIME type detection and validation.
//
// Role in project: Input structure for OCR tasks.
//
// Example:
//
//	imageData, _ := os.ReadFile("document.jpg")
//	ocrReq, err := types.NewOCRRequest(imageData)
type OCRRequest struct {
	Payload              []byte  `json:"payload"`
	PayloadMime          string  `json:"payload_mime_type"`
	DetectionThreshold   float32 `json:"detection_threshold,omitempty"`
	RecognitionThreshold float32 `json:"recognition_threshold,omitempty"`
	UseAngleCls          bool    `json:"use_angle_cls,omitempty"`
}

type OCRRequestOption func(*OCRRequest)

func WithDetectionThreshold(threshold float32) OCRRequestOption {
	return func(req *OCRRequest) {
		req.DetectionThreshold = threshold
	}
}

func WithRecognitionThreshold(threshold float32) OCRRequestOption {
	return func(req *OCRRequest) {
		req.RecognitionThreshold = threshold
	}
}

func WithUseAngleCls(useAngleCls bool) OCRRequestOption {
	return func(req *OCRRequest) {
		req.UseAngleCls = useAngleCls
	}
}

// NewOCRRequest creates a new OCR request.
//
// This function analyzes the payload to detect the image format and validates it's
// a supported type.
//
// Parameters:
//   - payload: The raw image bytes to process
//
// Returns:
//   - *OCRRequest: Configured request ready for ForOCR()
//   - error: Non-nil if the payload is not a supported image type
//
// Role in project: Factory function for creating OCR requests.
//
// Example:
//
//	imageData, _ := os.ReadFile("receipt.jpg")
//	ocrReq, err := types.NewOCRRequest(imageData)
//	if err != nil {
//	    log.Fatalf("Invalid image: %v", err)
//	}
//
//	inferReq := types.NewInferRequest("ocr").
//	    ForOCR(ocrReq, "ocr_model").
//	    Build()
func NewOCRRequest(payload []byte, opts ...OCRRequestOption) (*OCRRequest, error) {
	mime := mimetype.Detect(payload)
	mimeString := mime.String()

	if mimetype.EqualsAny(mimeString, SupportedImageMimeTypes...) {
		req := &OCRRequest{
			Payload:     payload,
			PayloadMime: mimeString,
		}
		for _, opt := range opts {
			opt(req)
		}
		return req, nil
	}
	return nil, fmt.Errorf("unsupported payload type: %s", mimeString)
}
