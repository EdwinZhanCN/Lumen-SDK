package types_test

import (
	"testing"

	"github.com/edwinzhancn/lumen-sdk/pkg/types"
)

func TestNewInferRequest(t *testing.T) {
	task := "test_task"
	builder := types.NewInferRequest(task)

	req := builder.Build()
	if req.Task != task {
		t.Errorf("Expected task %s, got %s", task, req.Task)
	}

	if req.Meta == nil {
		t.Error("Expected Meta to be initialized")
	}

	if req.PayloadMime != "application/json" {
		t.Errorf("Expected default PayloadMime 'application/json', got %s", req.PayloadMime)
	}
}

func TestInferRequestBuilderWithCorrelationID(t *testing.T) {
	correlationID := "test-correlation-id"
	builder := types.NewInferRequest("test_task").WithCorrelationID(correlationID)

	req := builder.Build()
	if req.CorrelationId != correlationID {
		t.Errorf("Expected CorrelationId %s, got %s", correlationID, req.CorrelationId)
	}
}

func TestInferRequestBuilderWithMeta(t *testing.T) {
	builder := types.NewInferRequest("test_task").
		WithMeta("key1", "value1").
		WithMeta("key2", "value2")

	req := builder.Build()

	if req.Meta["key1"] != "value1" {
		t.Errorf("Expected Meta['key1'] = 'value1', got %s", req.Meta["key1"])
	}

	if req.Meta["key2"] != "value2" {
		t.Errorf("Expected Meta['key2'] = 'value2', got %s", req.Meta["key2"])
	}
}

func TestInferRequestBuilderWithMetaOverwrite(t *testing.T) {
	builder := types.NewInferRequest("test_task").
		WithMeta("key1", "value1").
		WithMeta("key1", "value2")

	req := builder.Build()

	if req.Meta["key1"] != "value2" {
		t.Errorf("Expected Meta['key1'] = 'value2' (overwritten), got %s", req.Meta["key1"])
	}
}

func TestInferRequestBuilderForEmbedding(t *testing.T) {
	payload := []byte("test payload")
	embReq := &types.EmbeddingRequest{
		Payload:     payload,
		PayloadMime: "text/plain",
	}

	task := "embedding_task"
	builder := types.NewInferRequest("").ForEmbedding(embReq, task)

	req := builder.Build()

	if req.Task != task {
		t.Errorf("Expected task %s, got %s", task, req.Task)
	}

	if string(req.Payload) != string(payload) {
		t.Errorf("Expected payload %s, got %s", string(payload), string(req.Payload))
	}

	if req.PayloadMime != "text/plain" {
		t.Errorf("Expected PayloadMime 'text/plain', got %s", req.PayloadMime)
	}
}

func TestInferRequestBuilderForClassification(t *testing.T) {
	payload := []byte("test image data")
	classReq := &types.ClassificationRequest{
		Payload:     payload,
		PayloadMime: "image/jpeg",
	}

	task := "classification_task"
	builder := types.NewInferRequest("").ForClassification(classReq, task)

	req := builder.Build()

	if req.Task != task {
		t.Errorf("Expected task %s, got %s", task, req.Task)
	}

	if string(req.Payload) != string(payload) {
		t.Errorf("Expected payload %s, got %s", string(payload), string(req.Payload))
	}

	if req.PayloadMime != "image/jpeg" {
		t.Errorf("Expected PayloadMime 'image/jpeg', got %s", req.PayloadMime)
	}
}

func TestInferRequestBuilderForFaceDetection(t *testing.T) {
	payload := []byte("test image data")
	faceReq := &types.FaceRecognitionRequest{
		Payload:                      payload,
		PayloadMime:                  "image/jpeg",
		DetectionConfidenceThreshold: 0.85,
		NmsThreshold:                 0.4,
		FaceSizeMin:                  30.0,
		FaceSizeMax:                  500.0,
		MaxFaces:                     10,
	}

	task := "face_detection_task"
	builder := types.NewInferRequest("").ForFaceDetection(faceReq, task)

	req := builder.Build()

	if req.Task != task {
		t.Errorf("Expected task %s, got %s", task, req.Task)
	}

	// Check meta values
	if req.Meta["detection_confidence_threshold"] != "0.850" {
		t.Errorf("Expected detection_confidence_threshold meta '0.850', got %s", req.Meta["detection_confidence_threshold"])
	}

	if req.Meta["nms_threshold"] != "0.400" {
		t.Errorf("Expected nms_threshold meta '0.400', got %s", req.Meta["nms_threshold"])
	}

	if req.Meta["face_size_min"] != "30.0" {
		t.Errorf("Expected face_size_min meta '30.0', got %s", req.Meta["face_size_min"])
	}

	if req.Meta["face_size_max"] != "500.0" {
		t.Errorf("Expected face_size_max meta '500.0', got %s", req.Meta["face_size_max"])
	}

	if req.Meta["max_faces"] != "10" {
		t.Errorf("Expected max_faces meta '10', got %s", req.Meta["max_faces"])
	}
}

func TestInferRequestBuilderForFaceDetectionWithZeroValues(t *testing.T) {
	payload := []byte("test image data")
	faceReq := &types.FaceRecognitionRequest{
		Payload:     payload,
		PayloadMime: "image/jpeg",
		// All thresholds are zero - should not be set in meta
	}

	task := "face_detection_task"
	builder := types.NewInferRequest("").ForFaceDetection(faceReq, task)

	req := builder.Build()

	// Meta should not contain zero values
	if _, exists := req.Meta["detection_confidence_threshold"]; exists {
		t.Error("Expected detection_confidence_threshold not to be set for zero value")
	}

	if _, exists := req.Meta["nms_threshold"]; exists {
		t.Error("Expected nms_threshold not to be set for zero value")
	}

	if _, exists := req.Meta["face_size_min"]; exists {
		t.Error("Expected face_size_min not to be set for zero value")
	}

	if _, exists := req.Meta["face_size_max"]; exists {
		t.Error("Expected face_size_max not to be set for zero value")
	}
}

func TestInferRequestBuilderForFaceDetectionMaxFacesSpecialValues(t *testing.T) {
	payload := []byte("test image data")

	// Test MaxFaces = -1 (no limit)
	faceReq1 := &types.FaceRecognitionRequest{
		Payload:     payload,
		PayloadMime: "image/jpeg",
		MaxFaces:    -1,
	}

	builder1 := types.NewInferRequest("").ForFaceDetection(faceReq1, "task1")
	req1 := builder1.Build()

	if req1.Meta["max_faces"] != "-1" {
		t.Errorf("Expected max_faces meta '-1', got %s", req1.Meta["max_faces"])
	}

	// Test MaxFaces = 0 (should not be set)
	faceReq2 := &types.FaceRecognitionRequest{
		Payload:     payload,
		PayloadMime: "image/jpeg",
		MaxFaces:    0,
	}

	builder2 := types.NewInferRequest("").ForFaceDetection(faceReq2, "task2")
	req2 := builder2.Build()

	if _, exists := req2.Meta["max_faces"]; exists {
		t.Error("Expected max_faces not to be set for zero value")
	}

	// Test MaxFaces = -2 (invalid, should not be set)
	faceReq3 := &types.FaceRecognitionRequest{
		Payload:     payload,
		PayloadMime: "image/jpeg",
		MaxFaces:    -2,
	}

	builder3 := types.NewInferRequest("").ForFaceDetection(faceReq3, "task3")
	req3 := builder3.Build()

	if _, exists := req3.Meta["max_faces"]; exists {
		t.Error("Expected max_faces not to be set for invalid value -2")
	}
}

func TestInferRequestBuilderChaining(t *testing.T) {
	payload := []byte("test payload")
	embReq := &types.EmbeddingRequest{
		Payload:     payload,
		PayloadMime: "text/plain",
	}

	builder := types.NewInferRequest("").
		WithCorrelationID("test-id").
		WithMeta("custom_key", "custom_value").
		ForEmbedding(embReq, "embedding_task")

	req := builder.Build()

	if req.CorrelationId != "test-id" {
		t.Errorf("Expected CorrelationId 'test-id', got %s", req.CorrelationId)
	}

	if req.Meta["custom_key"] != "custom_value" {
		t.Errorf("Expected Meta['custom_key'] = 'custom_value', got %s", req.Meta["custom_key"])
	}

	if req.Task != "embedding_task" {
		t.Errorf("Expected task 'embedding_task', got %s", req.Task)
	}
}

func TestInferRequestBuilderForOCR(t *testing.T) {
	payload := []byte{0xFF, 0xD8, 0xFF, 0xE0} // Fake JPEG
	ocrReq := &types.OCRRequest{
		Payload:              payload,
		PayloadMime:          "image/jpeg",
		DetectionThreshold:   0.6,
		RecognitionThreshold: 0.7,
		UseAngleCls:          true,
	}

	task := "ocr_task"
	builder := types.NewInferRequest("").ForOCR(ocrReq, task)

	req := builder.Build()

	if req.Task != task {
		t.Errorf("Expected task %s, got %s", task, req.Task)
	}

	if req.Meta["detection_threshold"] != "0.600" {
		t.Errorf("Expected detection_threshold meta '0.600', got %s", req.Meta["detection_threshold"])
	}
	if req.Meta["recognition_threshold"] != "0.700" {
		t.Errorf("Expected recognition_threshold meta '0.700', got %s", req.Meta["recognition_threshold"])
	}
	if req.Meta["use_angle_cls"] != "true" {
		t.Errorf("Expected use_angle_cls meta 'true', got %s", req.Meta["use_angle_cls"])
	}
}
