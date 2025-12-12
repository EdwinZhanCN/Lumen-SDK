package types_test

import (
	"testing"

	"github.com/edwinzhancn/lumen-sdk/pkg/types"
)

func TestNewOCRRequest(t *testing.T) {
	// Minimal JPEG header
	payload := []byte{0xFF, 0xD8, 0xFF, 0xE0}

	req, err := types.NewOCRRequest(payload)
	if err != nil {
		t.Fatalf("NewOCRRequest failed: %v", err)
	}

	if req.PayloadMime != "image/jpeg" {
		t.Errorf("Expected mime image/jpeg, got %s", req.PayloadMime)
	}

	// Verify default values (zero values for float/bool)
	if req.DetectionThreshold != 0 {
		t.Errorf("Expected default DetectionThreshold 0, got %f", req.DetectionThreshold)
	}
	if req.UseAngleCls {
		t.Error("Expected default UseAngleCls false, got true")
	}
}

func TestNewOCRRequestWithOptions(t *testing.T) {
	// Minimal JPEG header
	payload := []byte{0xFF, 0xD8, 0xFF, 0xE0}

	req, err := types.NewOCRRequest(payload,
		types.WithDetectionThreshold(0.75),
		types.WithRecognitionThreshold(0.85),
		types.WithUseAngleCls(true),
	)
	if err != nil {
		t.Fatalf("NewOCRRequest failed: %v", err)
	}

	if req.DetectionThreshold != 0.75 {
		t.Errorf("Expected DetectionThreshold 0.75, got %f", req.DetectionThreshold)
	}

	if req.RecognitionThreshold != 0.85 {
		t.Errorf("Expected RecognitionThreshold 0.85, got %f", req.RecognitionThreshold)
	}

	if !req.UseAngleCls {
		t.Error("Expected UseAngleCls true, got false")
	}
}

func TestNewOCRRequestInvalidPayload(t *testing.T) {
	payload := []byte("not an image")

	_, err := types.NewOCRRequest(payload)
	if err == nil {
		t.Error("Expected error for invalid payload, got nil")
	}
}
