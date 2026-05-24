package types_test

import (
	"strings"
	"testing"

	"github.com/edwinzhancn/lumen-sdk/pkg/types"
)

func TestTaskContractRawBuilders(t *testing.T) {
	textReq := types.NewInferRequest("").ForSemanticTextEmbed("a cat", types.ServiceCLIP).Build()
	if textReq.Task != types.TaskSemanticTextEmbed || textReq.PayloadMime != "text/plain" || textReq.Meta[types.MetaService] != types.ServiceCLIP {
		t.Fatalf("unexpected text request: %+v", textReq)
	}
	if err := types.ValidateTaskRequest(textReq); err != nil {
		t.Fatalf("ValidateTaskRequest(text) error = %v", err)
	}

	imageReq := types.NewInferRequest("").ForSemanticImageEmbed([]byte("fake"), "image/avif", types.ServiceSigLIP).Build()
	if imageReq.Task != types.TaskSemanticImageEmbed || imageReq.PayloadMime != "image/avif" || imageReq.Meta[types.MetaService] != types.ServiceSigLIP {
		t.Fatalf("unexpected image request: %+v", imageReq)
	}
	if err := types.ValidateTaskRequest(imageReq); err != nil {
		t.Fatalf("ValidateTaskRequest(image) error = %v", err)
	}
}

func TestTaskContractTensorBuilders(t *testing.T) {
	req := types.NewInferRequest("").
		ForFaceRecognitionTensor(make([]byte, 1*3*640*640*4), "fp32", 640, 640, 1920, 1080, 0.3333333, 0, 140).
		Build()

	if req.Task != types.TaskFaceRecognition || req.PayloadMime != types.DefaultTensorMIME {
		t.Fatalf("unexpected face tensor request: %+v", req)
	}
	if req.Meta[types.MetaPreprocessID] != types.PreprocessInsightFaceDet || req.Meta[types.MetaService] != types.ServiceFace {
		t.Fatalf("unexpected tensor meta: %#v", req.Meta)
	}
	if err := types.ValidateTaskRequest(req); err != nil {
		t.Fatalf("ValidateTaskRequest(face tensor) error = %v", err)
	}
}

func TestTaskContractRejectsInvalidInputs(t *testing.T) {
	textTensor := types.NewInferRequest(types.TaskSemanticTextEmbed).
		ForTensorInput(make([]byte, 1*3*224*224*4), "", types.TensorDescriptor{
			DType:        "fp32",
			Shape:        []int64{1, 3, 224, 224},
			Layout:       "NCHW",
			PreprocessID: types.PreprocessCLIPImage,
		}).
		WithService(types.ServiceCLIP).
		Build()
	if err := types.ValidateTaskRequest(textTensor); err == nil || !strings.Contains(err.Error(), "does not support tensor") {
		t.Fatalf("expected text tensor rejection, got %v", err)
	}

	ocrMissingSource := types.NewInferRequest("").
		ForTensorInput(make([]byte, 1*3*736*1280*4), "", types.TensorDescriptor{
			DType:        "fp32",
			Shape:        []int64{1, 3, 736, 1280},
			Layout:       "NCHW",
			PreprocessID: types.PreprocessPPOCRDetection,
		}).
		WithService(types.ServiceOCR).
		Build()
	ocrMissingSource.Task = types.TaskOCR
	if err := types.ValidateTaskRequest(ocrMissingSource); err == nil || !strings.Contains(err.Error(), types.MetaSourceWidth) {
		t.Fatalf("expected OCR source metadata rejection, got %v", err)
	}

	deprecated := types.NewInferRequest(types.TaskFaceRecognition).
		WithPayload([]byte("{}"), types.DeprecatedTensorJSONMIME).
		WithService(types.ServiceFace).
		Build()
	if err := types.ValidateTaskRequest(deprecated); err == nil || !strings.Contains(err.Error(), "deprecated") {
		t.Fatalf("expected deprecated tensor JSON rejection, got %v", err)
	}
}

func TestTensorBatchingKey(t *testing.T) {
	req := types.NewInferRequest("").
		ForSemanticImageTensor(make([]byte, 2*3*224*224*4), types.ServiceCLIP, "fp32").
		Build()
	req.Meta[types.MetaTensorShape] = "[2,3,224,224]"
	req.Payload = make([]byte, 2*3*224*224*4)

	if err := types.ValidateTaskRequest(req); err != nil {
		t.Fatalf("ValidateTaskRequest(batch image tensor) error = %v", err)
	}
	key, ok, err := types.TensorBatchingKey(req)
	if err != nil || !ok {
		t.Fatalf("TensorBatchingKey() = %q, %v, %v", key, ok, err)
	}
	if !strings.Contains(key, "clip|semantic_image_embed|") || !strings.Contains(key, "3,224,224") {
		t.Fatalf("unexpected batching key: %s", key)
	}
}
