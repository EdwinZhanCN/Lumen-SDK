package types_test

import (
	"strings"
	"testing"

	"github.com/edwinzhancn/lumen-sdk/pkg/types"
)

func TestValidateTensorFastPathValidCLIPTensor(t *testing.T) {
	req := types.NewInferRequest("clip_embed").
		ForTensorInput(make([]byte, 1*3*224*224*4), "", types.TensorDescriptor{
			DType:        "fp32",
			Shape:        []int64{1, 3, 224, 224},
			Layout:       "NCHW",
			PreprocessID: "clip_image_openai_v1",
		}).
		Build()

	desc, err := types.ValidateTensorFastPath(req, types.TensorValidationOptions{
		AllowedPreprocessIDs: []string{"clip_image_openai_v1"},
	})
	if err != nil {
		t.Fatalf("ValidateTensorFastPath() error = %v", err)
	}
	if desc == nil {
		t.Fatal("Expected parsed tensor descriptor")
	}
	if desc.DType != "fp32" || desc.Layout != "NCHW" {
		t.Fatalf("Unexpected descriptor: %+v", desc)
	}
}

func TestValidateTensorFastPathRawRequestNoop(t *testing.T) {
	req := types.NewInferRequest("clip_embed").
		WithInputKind(types.InputKindRaw).
		Build()

	desc, err := types.ValidateTensorFastPath(req, types.TensorValidationOptions{})
	if err != nil {
		t.Fatalf("ValidateTensorFastPath() error = %v", err)
	}
	if desc != nil {
		t.Fatalf("Expected raw request to skip tensor validation, got %+v", desc)
	}
}

func TestValidateTensorFastPathByteLengthMismatch(t *testing.T) {
	req := types.NewInferRequest("clip_embed").
		ForTensorInput(make([]byte, 16), "", types.TensorDescriptor{
			DType:        "fp32",
			Shape:        []int64{1, 3, 224, 224},
			Layout:       "NCHW",
			PreprocessID: "clip_image_openai_v1",
		}).
		Build()

	_, err := types.ValidateTensorFastPath(req, types.TensorValidationOptions{})
	if err == nil || !strings.Contains(err.Error(), "payload length mismatch") {
		t.Fatalf("Expected payload length mismatch, got %v", err)
	}
}

func TestValidateTensorFastPathUnknownPreprocessID(t *testing.T) {
	req := types.NewInferRequest("clip_embed").
		ForTensorInput(make([]byte, 1*3*224*224*4), "", types.TensorDescriptor{
			DType:        "fp32",
			Shape:        []int64{1, 3, 224, 224},
			Layout:       "NCHW",
			PreprocessID: "unknown_preprocess",
		}).
		Build()

	_, err := types.ValidateTensorFastPath(req, types.TensorValidationOptions{
		AllowedPreprocessIDs: []string{"clip_image_openai_v1"},
	})
	if err == nil || !strings.Contains(err.Error(), types.MetaPreprocessID) {
		t.Fatalf("Expected preprocess id validation error, got %v", err)
	}
}

func TestValidateTensorFastPathMissingDescriptorFields(t *testing.T) {
	req := types.NewInferRequest("clip_embed").
		WithInputKind(types.InputKindTensor).
		Build()

	_, err := types.ValidateTensorFastPath(req, types.TensorValidationOptions{})
	if err == nil || !strings.Contains(err.Error(), types.MetaTensorDType) {
		t.Fatalf("Expected missing dtype error, got %v", err)
	}
}

func TestValidateTensorFastPathRejectsPayloadBatch(t *testing.T) {
	req := types.NewInferRequest("clip_embed").
		ForTensorInput(make([]byte, 2*3*224*224*4), "", types.TensorDescriptor{
			DType:        "fp32",
			Shape:        []int64{2, 3, 224, 224},
			Layout:       "NCHW",
			PreprocessID: "clip_image_openai_v1",
		}).
		Build()

	_, err := types.ValidateTensorFastPath(req, types.TensorValidationOptions{})
	if err == nil || !strings.Contains(err.Error(), "payload batching is not supported") {
		t.Fatalf("Expected payload batching rejection, got %v", err)
	}
}

func TestValidateTensorFastPathAllowsUnbatchedCHW(t *testing.T) {
	req := types.NewInferRequest("clip_embed").
		ForTensorInput(make([]byte, 3*224*224*4), "", types.TensorDescriptor{
			DType:        "fp32",
			Shape:        []int64{3, 224, 224},
			Layout:       "CHW",
			PreprocessID: "clip_image_openai_v1",
		}).
		Build()

	if _, err := types.ValidateTensorFastPath(req, types.TensorValidationOptions{}); err != nil {
		t.Fatalf("ValidateTensorFastPath() error = %v", err)
	}
}
