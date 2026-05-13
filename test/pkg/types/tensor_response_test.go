package types_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/edwinzhancn/lumen-sdk/pkg/types"
	pb "github.com/edwinzhancn/lumen-sdk/proto"
)

func TestValidateTensorResponseValid(t *testing.T) {
	resp := validTensorResponse([]byte{1, 2, 3, 4}, []int64{1, 4})

	desc, err := types.ValidateTensorResponse(resp, types.TensorOutputValidationOptions{})
	if err != nil {
		t.Fatalf("ValidateTensorResponse() error = %v", err)
	}
	if desc == nil {
		t.Fatal("expected tensor descriptor")
	}
	if desc.DType != "uint8" || desc.Layout != "NCHW" {
		t.Fatalf("unexpected descriptor: %+v", desc)
	}
}

func TestParseInferResponseAsTensorResponse(t *testing.T) {
	resp := validTensorResponse([]byte{1, 2, 3, 4}, []int64{1, 4})

	tensorResp, err := types.ParseInferResponse(resp).AsTensorResponse()
	if err != nil {
		t.Fatalf("AsTensorResponse() error = %v", err)
	}
	if string(tensorResp.Data) != string(resp.Result) {
		t.Fatalf("expected tensor data %v, got %v", resp.Result, tensorResp.Data)
	}
	if tensorResp.Descriptor.ModelID != "clip_vision_encoder" {
		t.Fatalf("expected model id clip_vision_encoder, got %s", tensorResp.Descriptor.ModelID)
	}
}

func TestValidateTensorResponseMissingOutputFields(t *testing.T) {
	resp := &pb.InferResponse{
		Result:     []byte{1, 2, 3, 4},
		ResultMime: types.DefaultTensorMIME,
		Meta: map[string]string{
			types.MetaOutputKind: types.OutputKindTensor,
		},
	}

	_, err := types.ValidateTensorResponse(resp, types.TensorOutputValidationOptions{})
	if err == nil || !strings.Contains(err.Error(), types.MetaOutputTensorDType) {
		t.Fatalf("expected missing output dtype error, got %v", err)
	}
}

func TestValidateTensorResponseByteLengthMismatch(t *testing.T) {
	resp := validTensorResponse([]byte{1, 2, 3}, []int64{1, 4})

	_, err := types.ValidateTensorResponse(resp, types.TensorOutputValidationOptions{})
	if err == nil || !strings.Contains(err.Error(), "payload length mismatch") {
		t.Fatalf("expected payload length mismatch, got %v", err)
	}
}

func TestValidateTensorResponseRawNoop(t *testing.T) {
	resp := &pb.InferResponse{
		Result:     []byte("{}"),
		ResultMime: "application/json;schema=embedding_v1",
		Meta: map[string]string{
			types.MetaOutputKind: types.OutputKindRaw,
		},
	}

	desc, err := types.ValidateTensorResponse(resp, types.TensorOutputValidationOptions{})
	if err != nil {
		t.Fatalf("ValidateTensorResponse() error = %v", err)
	}
	if desc != nil {
		t.Fatalf("expected raw response to skip tensor validation, got %+v", desc)
	}
}

func validTensorResponse(result []byte, shape []int64) *pb.InferResponse {
	return &pb.InferResponse{
		Result:     result,
		ResultMime: types.DefaultTensorMIME,
		IsFinal:    true,
		Meta: map[string]string{
			types.MetaOutputKind:            types.OutputKindTensor,
			types.MetaOutputTensorDType:     "uint8",
			types.MetaOutputTensorShape:     shapeJSON(shape),
			types.MetaOutputTensorLayout:    "NCHW",
			types.MetaOutputTensorFormat:    types.TensorFormatContig,
			types.MetaOutputTensorByteOrder: types.TensorByteOrderLittle,
			types.MetaModelID:               "clip_vision_encoder",
		},
	}
}

func shapeJSON(shape []int64) string {
	encoded, _ := json.Marshal(shape)
	return string(encoded)
}
