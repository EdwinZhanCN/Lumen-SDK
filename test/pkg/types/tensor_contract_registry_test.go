package types_test

import (
	"context"
	"testing"

	"github.com/edwinzhancn/lumen-sdk/pkg/types"
	pb "github.com/edwinzhancn/lumen-sdk/proto"
)

func TestTaskContractTensorHelpers(t *testing.T) {
	contract := types.NewTaskContract(&pb.IOTask{
		Name:                    types.TaskSemanticImageEmbed,
		TensorPreprocessId:      types.PreprocessSigLIPImage,
		TensorBatchingSupported: true,
	})

	if !contract.HasTensorPath() {
		t.Fatalf("expected tensor path")
	}
	if contract.TensorPreprocessID() != types.PreprocessSigLIPImage {
		t.Fatalf("unexpected preprocess id: %q", contract.TensorPreprocessID())
	}
	if !contract.TensorBatchingSupported() {
		t.Fatalf("expected batching supported")
	}

	if types.NewTaskContract(&pb.IOTask{Name: types.TaskOCR}).HasTensorPath() {
		t.Fatalf("empty preprocess id should not expose a tensor path")
	}
}

func TestDefaultTensorPreprocessorRegistry(t *testing.T) {
	registry := types.DefaultTensorPreprocessorRegistry()
	preprocessor, ok := registry.Lookup(types.PreprocessBioCLIP224Image)
	if !ok {
		t.Fatalf("expected BioCLIP preprocessor to be registered")
	}
	if _, ok := registry.Lookup("unknown_preprocess_v1"); ok {
		t.Fatalf("unknown preprocess id should not be registered")
	}

	input := types.ImageInput{
		Data:       make([]byte, 224*224*3),
		Width:      224,
		Height:     224,
		Channels:   3,
		Layout:     "HWC",
		DType:      "uint8",
		ColorSpace: "RGB",
	}
	payload, err := preprocessor.Preprocess(context.Background(), input)
	if err != nil {
		t.Fatalf("Preprocess() error = %v", err)
	}
	if payload.Descriptor.PreprocessID != types.PreprocessBioCLIP224Image {
		t.Fatalf("unexpected payload preprocess id: %q", payload.Descriptor.PreprocessID)
	}
	if got, want := len(payload.Payload), 1*3*224*224*4; got != want {
		t.Fatalf("payload length = %d, want %d", got, want)
	}
}
