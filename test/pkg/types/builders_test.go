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

	if req.PayloadMime != "" {
		t.Errorf("Expected empty default PayloadMime, got %s", req.PayloadMime)
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

func TestInferRequestBuilderTensorHelpers(t *testing.T) {
	payload := make([]byte, 1*3*224*224*4)
	builder := types.NewInferRequest(types.TaskSemanticImageEmbed).
		WithCorrelationID("tensor-1").
		ForTensorInput(payload, "", types.TensorDescriptor{
			DType:        "FP32",
			Shape:        []int64{1, 3, 224, 224},
			Layout:       "nchw",
			PreprocessID: types.PreprocessCLIPImage,
			ModelID:      "clip_vision_encoder",
			ModelVersion: "v1",
		})

	req := builder.Build()

	if req.PayloadMime != types.DefaultTensorMIME {
		t.Errorf("Expected tensor MIME %s, got %s", types.DefaultTensorMIME, req.PayloadMime)
	}
	if req.Meta[types.MetaInputKind] != types.InputKindTensor {
		t.Errorf("Expected tensor input kind, got %s", req.Meta[types.MetaInputKind])
	}
	if req.Meta[types.MetaTensorDType] != "fp32" {
		t.Errorf("Expected normalized dtype fp32, got %s", req.Meta[types.MetaTensorDType])
	}
	if req.Meta[types.MetaTensorShape] != "[1,3,224,224]" {
		t.Errorf("Expected tensor shape [1,3,224,224], got %s", req.Meta[types.MetaTensorShape])
	}
	if req.Meta[types.MetaTensorLayout] != "NCHW" {
		t.Errorf("Expected normalized layout NCHW, got %s", req.Meta[types.MetaTensorLayout])
	}
	if req.Meta[types.MetaTensorFormat] != types.TensorFormatContig {
		t.Errorf("Expected contiguous tensor format, got %s", req.Meta[types.MetaTensorFormat])
	}
	if req.Meta[types.MetaTensorByteOrder] != types.TensorByteOrderLittle {
		t.Errorf("Expected little byte order, got %s", req.Meta[types.MetaTensorByteOrder])
	}
	if req.Meta[types.MetaPreprocessID] != types.PreprocessCLIPImage {
		t.Errorf("Expected preprocess id, got %s", req.Meta[types.MetaPreprocessID])
	}
	if req.Meta[types.MetaPreprocessSkip] != "true" {
		t.Errorf("Expected preprocess skip true, got %s", req.Meta[types.MetaPreprocessSkip])
	}
	if req.Meta[types.MetaModelID] != "clip_vision_encoder" {
		t.Errorf("Expected model id, got %s", req.Meta[types.MetaModelID])
	}
	if req.Meta[types.MetaModelVersion] != "v1" {
		t.Errorf("Expected model version v1, got %s", req.Meta[types.MetaModelVersion])
	}
}

func TestInferRequestBuilderTensorPreprocessSkip(t *testing.T) {
	req := types.NewInferRequest(types.TaskSemanticImageEmbed).
		ForTensorInput([]byte{1}, "", types.TensorDescriptor{
			DType:          "uint8",
			Shape:          []int64{1},
			Layout:         "CHW",
			PreprocessSkip: true,
		}).
		Build()

	if req.Meta[types.MetaPreprocessSkip] != "true" {
		t.Fatalf("Expected preprocess skip true, got %s", req.Meta[types.MetaPreprocessSkip])
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
