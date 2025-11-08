package types_test

import (
	"testing"

	"github.com/edwinzhancn/lumen-sdk/pkg/types"
)

func TestNewFaceRecognitionRequestBasic(t *testing.T) {

	// Create a minimal JPEG header
	payload := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46}

	req, err := types.NewFaceRecognitionRequest(payload)
	if err != nil {
		t.Fatalf("NewFaceRecognitionRequest() error = %v", err)
	}

	if req.PayloadMime != "image/jpeg" {
		t.Errorf("Expected PayloadMime 'image/jpeg', got %s", req.PayloadMime)
	}

	if len(req.Payload) != len(payload) {
		t.Errorf("Expected Payload length %d, got %d", len(payload), len(req.Payload))
	}
}

func TestNewFaceRecognitionRequestWithOptions(t *testing.T) {

	payload := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46}

	req, err := types.NewFaceRecognitionRequest(payload,
		types.WithDetectionConfidenceThreshold(0.85),
		types.WithNmsThreshold(0.4),
		types.WithFaceSizeMin(30.0),
		types.WithFaceSizeMax(500.0),
		types.WithMaxFaces(10),
	)
	if err != nil {
		t.Fatalf("NewFaceRecognitionRequest() error = %v", err)
	}

	if req.DetectionConfidenceThreshold != 0.85 {
		t.Errorf("Expected DetectionConfidenceThreshold 0.85, got %f", req.DetectionConfidenceThreshold)
	}

	if req.NmsThreshold != 0.4 {
		t.Errorf("Expected NmsThreshold 0.4, got %f", req.NmsThreshold)
	}

	if req.FaceSizeMin != 30.0 {
		t.Errorf("Expected FaceSizeMin 30.0, got %f", req.FaceSizeMin)
	}

	if req.FaceSizeMax != 500.0 {
		t.Errorf("Expected FaceSizeMax 500.0, got %f", req.FaceSizeMax)
	}

	if req.MaxFaces != 10 {
		t.Errorf("Expected MaxFaces 10, got %d", req.MaxFaces)
	}
}

func TestWithDetectionConfidenceThreshold(t *testing.T) {

	payload := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46}

	req, err := types.NewFaceRecognitionRequest(payload,
		types.WithDetectionConfidenceThreshold(0.9),
	)
	if err != nil {
		t.Fatalf("NewFaceRecognitionRequest() error = %v", err)
	}

	if req.DetectionConfidenceThreshold != 0.9 {
		t.Errorf("Expected DetectionConfidenceThreshold 0.9, got %f", req.DetectionConfidenceThreshold)
	}
}

func TestWithNmsThreshold(t *testing.T) {

	payload := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46}

	req, err := types.NewFaceRecognitionRequest(payload,
		types.WithNmsThreshold(0.3),
	)
	if err != nil {
		t.Fatalf("NewFaceRecognitionRequest() error = %v", err)
	}

	if req.NmsThreshold != 0.3 {
		t.Errorf("Expected NmsThreshold 0.3, got %f", req.NmsThreshold)
	}
}

func TestWithFaceSizeMin(t *testing.T) {

	payload := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46}

	req, err := types.NewFaceRecognitionRequest(payload,
		types.WithFaceSizeMin(50.0),
	)
	if err != nil {
		t.Fatalf("NewFaceRecognitionRequest() error = %v", err)
	}

	if req.FaceSizeMin != 50.0 {
		t.Errorf("Expected FaceSizeMin 50.0, got %f", req.FaceSizeMin)
	}
}

func TestWithFaceSizeMax(t *testing.T) {

	payload := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46}

	req, err := types.NewFaceRecognitionRequest(payload,
		types.WithFaceSizeMax(1000.0),
	)
	if err != nil {
		t.Fatalf("NewFaceRecognitionRequest() error = %v", err)
	}

	if req.FaceSizeMax != 1000.0 {
		t.Errorf("Expected FaceSizeMax 1000.0, got %f", req.FaceSizeMax)
	}
}

func TestWithMaxFaces(t *testing.T) {

	payload := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46}

	req, err := types.NewFaceRecognitionRequest(payload,
		types.WithMaxFaces(5),
	)
	if err != nil {
		t.Fatalf("NewFaceRecognitionRequest() error = %v", err)
	}

	if req.MaxFaces != 5 {
		t.Errorf("Expected MaxFaces 5, got %d", req.MaxFaces)
	}
}

func TestWithMaxFacesNoLimit(t *testing.T) {

	payload := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46}

	req, err := types.NewFaceRecognitionRequest(payload,
		types.WithMaxFaces(-1),
	)
	if err != nil {
		t.Fatalf("NewFaceRecognitionRequest() error = %v", err)
	}

	if req.MaxFaces != -1 {
		t.Errorf("Expected MaxFaces -1 (no limit), got %d", req.MaxFaces)
	}
}

func TestNewFaceRecognitionRequestMultipleOptions(t *testing.T) {

	payload := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46}

	req, err := types.NewFaceRecognitionRequest(payload,
		types.WithDetectionConfidenceThreshold(0.75),
		types.WithNmsThreshold(0.5),
		types.WithFaceSizeMin(40.0),
	)
	if err != nil {
		t.Fatalf("NewFaceRecognitionRequest() error = %v", err)
	}

	if req.DetectionConfidenceThreshold != 0.75 {
		t.Errorf("Expected DetectionConfidenceThreshold 0.75, got %f", req.DetectionConfidenceThreshold)
	}

	if req.NmsThreshold != 0.5 {
		t.Errorf("Expected NmsThreshold 0.5, got %f", req.NmsThreshold)
	}

	if req.FaceSizeMin != 40.0 {
		t.Errorf("Expected FaceSizeMin 40.0, got %f", req.FaceSizeMin)
	}

	// Unset options should have zero values
	if req.FaceSizeMax != 0 {
		t.Errorf("Expected FaceSizeMax 0 (not set), got %f", req.FaceSizeMax)
	}

	if req.MaxFaces != 0 {
		t.Errorf("Expected MaxFaces 0 (not set), got %d", req.MaxFaces)
	}
}

func TestNewFaceRecognitionRequestUnsupportedType(t *testing.T) {
	// Plain text should not be supported
	payload := []byte("This is plain text")

	_, err := types.NewFaceRecognitionRequest(payload)
	if err == nil {
		t.Error("Expected error for unsupported payload type, got nil")
	}
}

func TestNewFaceRecognitionRequestEmptyPayload(t *testing.T) {
	payload := []byte{}

	_, err := types.NewFaceRecognitionRequest(payload)
	if err == nil {
		t.Error("Expected error for empty payload, got nil")
	}
}

func TestFaceV1Struct(t *testing.T) {
	face := types.Face{
		BBox:       []float32{10.0, 20.0, 100.0, 120.0},
		Confidence: 0.95,
		Landmarks:  []float32{30.0, 40.0, 70.0, 40.0},
		Embedding:  []float32{0.1, 0.2, 0.3},
	}

	if len(face.BBox) != 4 {
		t.Errorf("Expected BBox length 4, got %d", len(face.BBox))
	}

	if face.Confidence != 0.95 {
		t.Errorf("Expected Confidence 0.95, got %f", face.Confidence)
	}

	if len(face.Landmarks) != 4 {
		t.Errorf("Expected Landmarks length 4, got %d", len(face.Landmarks))
	}

	if len(face.Embedding) != 3 {
		t.Errorf("Expected Embedding length 3, got %d", len(face.Embedding))
	}
}

func TestFaceV1StructWithMultipleFaces(t *testing.T) {
	faceV1 := types.FaceV1{
		Faces: []types.Face{
			{
				BBox:       []float32{10.0, 20.0, 100.0, 120.0},
				Confidence: 0.95,
			},
			{
				BBox:       []float32{200.0, 50.0, 280.0, 150.0},
				Confidence: 0.88,
			},
		},
		Count:   2,
		ModelID: "face_detection_model",
	}

	if faceV1.Count != 2 {
		t.Errorf("Expected Count 2, got %d", faceV1.Count)
	}

	if len(faceV1.Faces) != 2 {
		t.Fatalf("Expected 2 faces, got %d", len(faceV1.Faces))
	}

	if faceV1.ModelID != "face_detection_model" {
		t.Errorf("Expected ModelID 'face_detection_model', got %s", faceV1.ModelID)
	}
}

func TestFaceRecognitionRequestStruct(t *testing.T) {
	req := types.FaceRecognitionRequest{
		Payload:                      []byte("test data"),
		PayloadMime:                  "image/jpeg",
		DetectionConfidenceThreshold: 0.8,
		NmsThreshold:                 0.4,
		FaceSizeMin:                  20.0,
		FaceSizeMax:                  800.0,
		MaxFaces:                     15,
	}

	if string(req.Payload) != "test data" {
		t.Errorf("Expected Payload 'test data', got %s", string(req.Payload))
	}

	if req.PayloadMime != "image/jpeg" {
		t.Errorf("Expected PayloadMime 'image/jpeg', got %s", req.PayloadMime)
	}

	if req.DetectionConfidenceThreshold != 0.8 {
		t.Errorf("Expected DetectionConfidenceThreshold 0.8, got %f", req.DetectionConfidenceThreshold)
	}

	if req.NmsThreshold != 0.4 {
		t.Errorf("Expected NmsThreshold 0.4, got %f", req.NmsThreshold)
	}

	if req.FaceSizeMin != 20.0 {
		t.Errorf("Expected FaceSizeMin 20.0, got %f", req.FaceSizeMin)
	}

	if req.FaceSizeMax != 800.0 {
		t.Errorf("Expected FaceSizeMax 800.0, got %f", req.FaceSizeMax)
	}

	if req.MaxFaces != 15 {
		t.Errorf("Expected MaxFaces 15, got %d", req.MaxFaces)
	}
}

func TestFaceOptionalFields(t *testing.T) {
	// Test Face with only required fields
	face := types.Face{
		BBox:       []float32{10.0, 20.0, 100.0, 120.0},
		Confidence: 0.95,
	}

	if len(face.BBox) != 4 {
		t.Errorf("Expected BBox length 4, got %d", len(face.BBox))
	}

	if face.Confidence != 0.95 {
		t.Errorf("Expected Confidence 0.95, got %f", face.Confidence)
	}

	// Optional fields should be nil/empty
	if face.Landmarks != nil && len(face.Landmarks) > 0 {
		t.Error("Expected Landmarks to be empty for optional field")
	}

	if face.Embedding != nil && len(face.Embedding) > 0 {
		t.Error("Expected Embedding to be empty for optional field")
	}
}
