package client_test

import (
	"context"
	"strings"
	"testing"

	"github.com/edwinzhancn/lumen-sdk/pkg/client"
	"github.com/edwinzhancn/lumen-sdk/pkg/config"
	"github.com/edwinzhancn/lumen-sdk/pkg/types"
	pb "github.com/edwinzhancn/lumen-sdk/proto"
)

func TestInferValidatesTensorFastPathBeforeRouting(t *testing.T) {
	c, err := client.NewLumenClient(config.DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("NewLumenClient() error = %v", err)
	}

	req := invalidTensorFastPathRequest()
	_, err = c.Infer(context.Background(), req)
	if err == nil || !strings.Contains(err.Error(), "payload length mismatch") {
		t.Fatalf("expected tensor validation error before routing, got %v", err)
	}
}

func TestInferStreamValidatesTensorFastPathBeforeRouting(t *testing.T) {
	c, err := client.NewLumenClient(config.DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("NewLumenClient() error = %v", err)
	}

	req := invalidTensorFastPathRequest()
	_, err = c.InferStream(context.Background(), req)
	if err == nil || !strings.Contains(err.Error(), "payload length mismatch") {
		t.Fatalf("expected tensor validation error before routing, got %v", err)
	}
}

func invalidTensorFastPathRequest() *pb.InferRequest {
	return types.NewInferRequest("clip_embed").
		ForTensorInput([]byte{1, 2, 3}, "", types.TensorDescriptor{
			DType:        "fp32",
			Shape:        []int64{1, 3, 224, 224},
			Layout:       "NCHW",
			PreprocessID: "clip_image_openai_v1",
		}).
		Build()
}
