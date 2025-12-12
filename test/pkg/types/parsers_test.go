package types_test

import (
	"encoding/json"
	"testing"

	"github.com/edwinzhancn/lumen-sdk/pkg/types"
	pb "github.com/edwinzhancn/lumen-sdk/proto"
)

func TestParseInferResponseAsFaceResponse(t *testing.T) {
	faceData := types.FaceV1{
		Faces: []types.Face{
			{
				BBox:       []float32{10, 20, 100, 120},
				Confidence: 0.95,
				Landmarks:  []float32{30, 40, 70, 40},
			},
		},
		Count:   1,
		ModelID: "face_model_1",
	}

	resultBytes, err := json.Marshal(faceData)
	if err != nil {
		t.Fatalf("Failed to marshal face data: %v", err)
	}

	resp := &pb.InferResponse{
		Result:     resultBytes,
		ResultMime: "application/json;schema=face_v1",
	}

	parser := types.ParseInferResponse(resp)
	parsedFace, err := parser.AsFaceResponse()
	if err != nil {
		t.Fatalf("AsFaceResponse() error = %v", err)
	}

	if parsedFace.Count != 1 {
		t.Errorf("Expected Count 1, got %d", parsedFace.Count)
	}

	if parsedFace.ModelID != "face_model_1" {
		t.Errorf("Expected ModelID 'face_model_1', got %s", parsedFace.ModelID)
	}

	if len(parsedFace.Faces) != 1 {
		t.Fatalf("Expected 1 face, got %d", len(parsedFace.Faces))
	}

	face := parsedFace.Faces[0]
	if face.Confidence != 0.95 {
		t.Errorf("Expected Confidence 0.95, got %f", face.Confidence)
	}
}

func TestParseInferResponseAsFaceResponseWrongMime(t *testing.T) {
	resp := &pb.InferResponse{
		Result:     []byte("{}"),
		ResultMime: "application/json;schema=embedding_v1",
	}

	parser := types.ParseInferResponse(resp)
	_, err := parser.AsFaceResponse()
	if err == nil {
		t.Error("Expected error for wrong MIME type, got nil")
	}
}

func TestParseInferResponseAsFaceResponseInvalidJSON(t *testing.T) {
	resp := &pb.InferResponse{
		Result:     []byte("invalid json"),
		ResultMime: "application/json;schema=face_v1",
	}

	parser := types.ParseInferResponse(resp)
	_, err := parser.AsFaceResponse()
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
}

func TestParseInferResponseAsEmbeddingResponse(t *testing.T) {
	embData := types.EmbeddingV1{
		Vector:  []float32{0.1, 0.2, 0.3, 0.4},
		Dim:     4,
		ModelID: "embedding_model_1",
	}

	resultBytes, err := json.Marshal(embData)
	if err != nil {
		t.Fatalf("Failed to marshal embedding data: %v", err)
	}

	resp := &pb.InferResponse{
		Result:     resultBytes,
		ResultMime: "application/json;schema=embedding_v1",
	}

	parser := types.ParseInferResponse(resp)
	parsedEmb, err := parser.AsEmbeddingResponse()
	if err != nil {
		t.Fatalf("AsEmbeddingResponse() error = %v", err)
	}

	if parsedEmb.Dim != 4 {
		t.Errorf("Expected Dim 4, got %d", parsedEmb.Dim)
	}

	if parsedEmb.ModelID != "embedding_model_1" {
		t.Errorf("Expected ModelID 'embedding_model_1', got %s", parsedEmb.ModelID)
	}

	if len(parsedEmb.Vector) != 4 {
		t.Fatalf("Expected 4 vector elements, got %d", len(parsedEmb.Vector))
	}

	for i, v := range []float32{0.1, 0.2, 0.3, 0.4} {
		if parsedEmb.Vector[i] != v {
			t.Errorf("Vector[%d]: expected %f, got %f", i, v, parsedEmb.Vector[i])
		}
	}
}

func TestParseInferResponseAsEmbeddingResponseWrongMime(t *testing.T) {
	resp := &pb.InferResponse{
		Result:     []byte("{}"),
		ResultMime: "application/json;schema=labels_v1",
	}

	parser := types.ParseInferResponse(resp)
	_, err := parser.AsEmbeddingResponse()
	if err == nil {
		t.Error("Expected error for wrong MIME type, got nil")
	}
}

func TestParseInferResponseAsClassificationResponse(t *testing.T) {
	labelsData := types.LabelsV1{
		Labels: []types.Label{
			{Label: "cat", Score: 0.9},
			{Label: "dog", Score: 0.1},
		},
		ModelID: "classification_model_1",
	}

	resultBytes, err := json.Marshal(labelsData)
	if err != nil {
		t.Fatalf("Failed to marshal labels data: %v", err)
	}

	resp := &pb.InferResponse{
		Result:     resultBytes,
		ResultMime: "application/json;schema=labels_v1",
	}

	parser := types.ParseInferResponse(resp)
	parsedLabels, err := parser.AsClassificationResponse()
	if err != nil {
		t.Fatalf("AsClassificationResponse() error = %v", err)
	}

	if parsedLabels.ModelID != "classification_model_1" {
		t.Errorf("Expected ModelID 'classification_model_1', got %s", parsedLabels.ModelID)
	}

	if len(parsedLabels.Labels) != 2 {
		t.Fatalf("Expected 2 labels, got %d", len(parsedLabels.Labels))
	}

	if parsedLabels.Labels[0].Label != "cat" {
		t.Errorf("Expected first label 'cat', got %s", parsedLabels.Labels[0].Label)
	}

	if parsedLabels.Labels[0].Score != 0.9 {
		t.Errorf("Expected first score 0.9, got %f", parsedLabels.Labels[0].Score)
	}
}

func TestParseInferResponseAsClassificationResponseWrongMime(t *testing.T) {
	resp := &pb.InferResponse{
		Result:     []byte("{}"),
		ResultMime: "application/json;schema=face_v1",
	}

	parser := types.ParseInferResponse(resp)
	_, err := parser.AsClassificationResponse()
	if err == nil {
		t.Error("Expected error for wrong MIME type, got nil")
	}
}

func TestParseInferResponseRaw(t *testing.T) {
	resp := &pb.InferResponse{
		Result:     []byte("test data"),
		ResultMime: "application/octet-stream",
		IsFinal:    true,
	}

	parser := types.ParseInferResponse(resp)
	raw := parser.Raw()

	if string(raw.Result) != "test data" {
		t.Errorf("Expected Result 'test data', got %s", string(raw.Result))
	}

	if raw.ResultMime != "application/octet-stream" {
		t.Errorf("Expected ResultMime 'application/octet-stream', got %s", raw.ResultMime)
	}

	if !raw.IsFinal {
		t.Error("Expected IsFinal true, got false")
	}
}

func TestParseInferResponseEmptyResponse(t *testing.T) {
	resp := &pb.InferResponse{
		Result:     []byte("{}"),
		ResultMime: "application/json;schema=embedding_v1",
	}

	parser := types.ParseInferResponse(resp)
	parsedEmb, err := parser.AsEmbeddingResponse()
	if err != nil {
		t.Fatalf("AsEmbeddingResponse() error = %v", err)
	}

	if parsedEmb.Dim != 0 {
		t.Errorf("Expected Dim 0 for empty response, got %d", parsedEmb.Dim)
	}

	if len(parsedEmb.Vector) != 0 {
		t.Errorf("Expected empty Vector, got %d elements", len(parsedEmb.Vector))
	}
}

func TestParseInferResponseNilResponse(t *testing.T) {
	parser := types.ParseInferResponse(nil)
	raw := parser.Raw()

	if raw != nil {
		t.Error("Expected nil Raw() result for nil response")
	}
}

func TestParseInferResponseAsOCRResponse(t *testing.T) {
	ocrData := types.OCRV1{
		Items: []types.OCRItem{
			{
				Box:        [][]int{{0, 0}, {10, 0}, {10, 10}, {0, 10}},
				Text:       "Hello",
				Confidence: 0.99,
			},
		},
		Count:   1,
		ModelID: "ocr_model_v1",
	}

	resultBytes, err := json.Marshal(ocrData)
	if err != nil {
		t.Fatalf("Failed to marshal OCR data: %v", err)
	}

	resp := &pb.InferResponse{
		Result:     resultBytes,
		ResultMime: "application/json;schema=ocr_v1",
	}

	parser := types.ParseInferResponse(resp)
	parsedOCR, err := parser.AsOCRResponse()
	if err != nil {
		t.Fatalf("AsOCRResponse() error = %v", err)
	}

	if parsedOCR.Count != 1 {
		t.Errorf("Expected Count 1, got %d", parsedOCR.Count)
	}
	if parsedOCR.ModelID != "ocr_model_v1" {
		t.Errorf("Expected ModelID 'ocr_model_v1', got %s", parsedOCR.ModelID)
	}
	if len(parsedOCR.Items) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(parsedOCR.Items))
	}
	if parsedOCR.Items[0].Text != "Hello" {
		t.Errorf("Expected text 'Hello', got %s", parsedOCR.Items[0].Text)
	}
}

func TestParseInferResponseAsOCRResponseWrongMime(t *testing.T) {
	resp := &pb.InferResponse{
		Result:     []byte("{}"),
		ResultMime: "application/json;schema=face_v1",
	}

	parser := types.ParseInferResponse(resp)
	_, err := parser.AsOCRResponse()
	if err == nil {
		t.Error("Expected error for wrong MIME type, got nil")
	}
}
