package types

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	pb "github.com/edwinzhancn/lumen-sdk/proto"
	"github.com/gabriel-vasile/mimetype"
)

const (
	TaskSemanticTextEmbed  = "semantic_text_embed"
	TaskSemanticImageEmbed = "semantic_image_embed"
	TaskBioCLIPClassify    = "bioclip_classify"
	TaskOCR                = "ocr"
	TaskFaceRecognition    = "face_recognition"

	ServiceCLIP    = "clip"
	ServiceBioCLIP = "bioclip"
	ServiceSigLIP  = "siglip"
	ServiceOCR     = "ocr"
	ServiceFace    = "face"

	PreprocessCLIPImage      = "clip_image_preprocess_v1"
	PreprocessSigLIPImage    = "siglip_image_preprocess_v1"
	PreprocessPPOCRDetection = "ppocr_det_v1"
	PreprocessInsightFaceDet = "insightface_det_v1"

	MetaService        = "service"
	MetaTopK           = "top_k"
	MetaSourceWidth    = "lumen.source.width"
	MetaSourceHeight   = "lumen.source.height"
	MetaLetterboxScale = "lumen.letterbox.scale"
	MetaLetterboxPadX  = "lumen.letterbox.pad_x"
	MetaLetterboxPadY  = "lumen.letterbox.pad_y"

	DeprecatedTensorJSONMIME = "application/vnd.lumen.tensor+json"
)

var TopKMetaAliases = []string{"TopK", "topK", "top_k", "top-k", "lumen.top_k"}

// WithService records the target Lumen Hub service in InferRequest.Meta.
func (b *InferRequestBuilder) WithService(service string) *InferRequestBuilder {
	return b.WithMeta(MetaService, strings.TrimSpace(service))
}

func (b *InferRequestBuilder) WithPayload(payload []byte, mime string) *InferRequestBuilder {
	b.req.Payload = payload
	b.req.PayloadMime = strings.TrimSpace(mime)
	return b
}

func (b *InferRequestBuilder) ForSemanticTextEmbed(text string, service string) *InferRequestBuilder {
	b.req.Task = TaskSemanticTextEmbed
	b.req.Payload = []byte(text)
	b.req.PayloadMime = "text/plain"
	return b.WithService(service)
}

func (b *InferRequestBuilder) ForSemanticImageEmbed(payload []byte, mime string, service string) *InferRequestBuilder {
	b.req.Task = TaskSemanticImageEmbed
	b.req.Payload = payload
	b.req.PayloadMime = strings.TrimSpace(mime)
	return b.WithService(service)
}

func (b *InferRequestBuilder) ForSemanticImageTensor(payload []byte, service string, dtype string) *InferRequestBuilder {
	preprocessID := PreprocessCLIPImage
	if service == ServiceSigLIP {
		preprocessID = PreprocessSigLIPImage
	}
	b.ForTensorInput(payload, DefaultTensorMIME, TensorDescriptor{
		DType:          dtype,
		Shape:          []int64{1, 3, 224, 224},
		Layout:         "NCHW",
		PreprocessID:   preprocessID,
		PreprocessSkip: true,
	}).WithService(service)
	b.req.Task = TaskSemanticImageEmbed
	return b
}

func (b *InferRequestBuilder) ForBioCLIPClassify(payload []byte, mime string, topK int) *InferRequestBuilder {
	b.req.Task = TaskBioCLIPClassify
	b.req.Payload = payload
	b.req.PayloadMime = strings.TrimSpace(mime)
	b.WithService(ServiceBioCLIP)
	if topK > 0 {
		b.WithMeta(MetaTopK, strconv.Itoa(topK))
	}
	return b
}

func (b *InferRequestBuilder) ForBioCLIPTensor(payload []byte, dtype string, topK int) *InferRequestBuilder {
	b.ForTensorInput(payload, DefaultTensorMIME, TensorDescriptor{
		DType:          dtype,
		Shape:          []int64{1, 3, 224, 224},
		Layout:         "NCHW",
		PreprocessID:   PreprocessCLIPImage,
		PreprocessSkip: true,
	}).WithService(ServiceCLIP)
	b.req.Task = TaskBioCLIPClassify
	if topK > 0 {
		b.WithMeta(MetaTopK, strconv.Itoa(topK))
	}
	return b
}

func (b *InferRequestBuilder) ForOCRRaw(payload []byte, mime string) *InferRequestBuilder {
	b.req.Task = TaskOCR
	b.req.Payload = payload
	b.req.PayloadMime = strings.TrimSpace(mime)
	return b.WithService(ServiceOCR)
}

func (b *InferRequestBuilder) ForOCRTensor(payload []byte, dtype string, h, w int64, sourceWidth, sourceHeight int) *InferRequestBuilder {
	b.ForTensorInput(payload, DefaultTensorMIME, TensorDescriptor{
		DType:          dtype,
		Shape:          []int64{1, 3, h, w},
		Layout:         "NCHW",
		PreprocessID:   PreprocessPPOCRDetection,
		PreprocessSkip: true,
	}).WithService(ServiceOCR)
	b.req.Task = TaskOCR
	b.WithMeta(MetaSourceWidth, strconv.Itoa(sourceWidth))
	b.WithMeta(MetaSourceHeight, strconv.Itoa(sourceHeight))
	return b
}

func (b *InferRequestBuilder) ForFaceRecognitionRaw(payload []byte, mime string) *InferRequestBuilder {
	b.req.Task = TaskFaceRecognition
	b.req.Payload = payload
	b.req.PayloadMime = strings.TrimSpace(mime)
	return b.WithService(ServiceFace)
}

func (b *InferRequestBuilder) ForFaceRecognitionTensor(payload []byte, dtype string, h, w int64, sourceWidth, sourceHeight int, scale, padX, padY float64) *InferRequestBuilder {
	b.ForTensorInput(payload, DefaultTensorMIME, TensorDescriptor{
		DType:          dtype,
		Shape:          []int64{1, 3, h, w},
		Layout:         "NCHW",
		PreprocessID:   PreprocessInsightFaceDet,
		PreprocessSkip: true,
	}).WithService(ServiceFace)
	b.req.Task = TaskFaceRecognition
	b.WithMeta(MetaSourceWidth, strconv.Itoa(sourceWidth))
	b.WithMeta(MetaSourceHeight, strconv.Itoa(sourceHeight))
	b.WithMeta(MetaLetterboxScale, strconv.FormatFloat(scale, 'f', -1, 64))
	b.WithMeta(MetaLetterboxPadX, strconv.FormatFloat(padX, 'f', -1, 64))
	b.WithMeta(MetaLetterboxPadY, strconv.FormatFloat(padY, 'f', -1, 64))
	return b
}

// ValidateTaskRequest validates the public Lumen Hub task request contract.
func ValidateTaskRequest(req *pb.InferRequest) error {
	if req == nil {
		return fmt.Errorf("request cannot be nil")
	}
	if strings.TrimSpace(req.Task) == "" {
		return fmt.Errorf("task is required")
	}
	mime := strings.TrimSpace(req.PayloadMime)
	if mime == "" {
		return fmt.Errorf("payload_mime is required")
	}
	if strings.EqualFold(mime, DeprecatedTensorJSONMIME) {
		return fmt.Errorf("%s is deprecated; use %s with tensor metadata", DeprecatedTensorJSONMIME, DefaultTensorMIME)
	}
	if mime == "application/json" && looksLikeJSONPixelPayload(req.Payload) {
		return fmt.Errorf("JSON pixel payloads are not supported; use %s with tensor metadata", DefaultTensorMIME)
	}

	isTensor := strings.EqualFold(mime, DefaultTensorMIME)
	switch req.Task {
	case TaskSemanticTextEmbed:
		if isTensor {
			return fmt.Errorf("%s does not support tensor input", TaskSemanticTextEmbed)
		}
		if mime != "text/plain" {
			return fmt.Errorf("%s requires text/plain payload_mime", TaskSemanticTextEmbed)
		}
	case TaskSemanticImageEmbed:
		if isTensor {
			return validateImageTensorTask(req, []string{PreprocessCLIPImage, PreprocessSigLIPImage}, true)
		}
		return validateRawImageMIME(mime)
	case TaskBioCLIPClassify:
		if service := ServiceFromMeta(req.Meta); service != "" && service != ServiceCLIP {
			return fmt.Errorf("%s requires service %q", TaskBioCLIPClassify, ServiceCLIP)
		}
		if isTensor {
			return validateImageTensorTask(req, []string{PreprocessCLIPImage}, true)
		}
		return validateRawImageMIME(mime)
	case TaskOCR:
		if isTensor {
			return validateDetTensorTask(req, PreprocessPPOCRDetection, false)
		}
		return validateRawImageMIME(mime)
	case TaskFaceRecognition:
		if isTensor {
			return validateDetTensorTask(req, PreprocessInsightFaceDet, true)
		}
		return validateRawImageMIME(mime)
	default:
		if _, err := ValidateTensorFastPath(req, TensorValidationOptions{}); err != nil {
			return err
		}
	}
	return nil
}

func ServiceFromMeta(meta map[string]string) string {
	if meta == nil {
		return ""
	}
	return strings.TrimSpace(meta[MetaService])
}

func TensorBatchingKey(req *pb.InferRequest) (string, bool, error) {
	desc, isTensor, err := ParseTensorDescriptor(req.GetMeta())
	if err != nil || !isTensor {
		return "", false, err
	}
	service := ServiceFromMeta(req.Meta)
	shapeTail := ""
	if len(desc.Shape) > 1 {
		values := make([]string, 0, len(desc.Shape)-1)
		for _, dim := range desc.Shape[1:] {
			values = append(values, strconv.FormatInt(dim, 10))
		}
		shapeTail = strings.Join(values, ",")
	}
	key := strings.Join([]string{service, req.Task, desc.ModelID, desc.DType, shapeTail, desc.PreprocessID}, "|")
	return key, true, nil
}

func DecodeRESTPayload(raw json.RawMessage, payloadMIME string) ([]byte, error) {
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		if payloadMIME == "text/plain" {
			return []byte(asString), nil
		}
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(asString))
		if err != nil {
			return nil, fmt.Errorf("payload must be base64 for %s: %w", payloadMIME, err)
		}
		return decoded, nil
	}
	var bytes []byte
	if err := json.Unmarshal(raw, &bytes); err == nil {
		return bytes, nil
	}
	return nil, fmt.Errorf("payload must be a string")
}

func validateImageTensorTask(req *pb.InferRequest, preprocessIDs []string, allowBatch bool) error {
	if _, err := ValidateTensorFastPath(req, TensorValidationOptions{
		AllowedDTypes:           []string{"fp32", "fp16"},
		AllowedLayouts:          []string{"NCHW"},
		AllowedPreprocessIDs:    preprocessIDs,
		DisableSingleBatchCheck: allowBatch,
	}); err != nil {
		return err
	}
	desc, _, _ := ParseTensorDescriptor(req.Meta)
	if len(desc.Shape) != 4 || desc.Shape[1] != 3 || desc.Shape[2] != 224 || desc.Shape[3] != 224 {
		return fmt.Errorf("%s must be [N,3,224,224]", MetaTensorShape)
	}
	service := ServiceFromMeta(req.Meta)
	switch desc.PreprocessID {
	case PreprocessCLIPImage:
		if service != "" && service != ServiceCLIP {
			return fmt.Errorf("%s requires service %q for %s", MetaPreprocessID, ServiceCLIP, PreprocessCLIPImage)
		}
	case PreprocessSigLIPImage:
		if service != "" && service != ServiceSigLIP {
			return fmt.Errorf("%s requires service %q for %s", MetaPreprocessID, ServiceSigLIP, PreprocessSigLIPImage)
		}
	}
	return nil
}

func validateDetTensorTask(req *pb.InferRequest, preprocessID string, requireLetterbox bool) error {
	if _, err := ValidateTensorFastPath(req, TensorValidationOptions{
		AllowedDTypes:        []string{"fp32"},
		AllowedLayouts:       []string{"NCHW"},
		AllowedPreprocessIDs: []string{preprocessID},
	}); err != nil {
		return err
	}
	desc, _, _ := ParseTensorDescriptor(req.Meta)
	if len(desc.Shape) != 4 || desc.Shape[0] != 1 || desc.Shape[1] != 3 {
		return fmt.Errorf("%s must be [1,3,H,W]", MetaTensorShape)
	}
	if preprocessID == PreprocessPPOCRDetection && (desc.Shape[2]%32 != 0 || desc.Shape[3]%32 != 0) {
		return fmt.Errorf("%s H and W must be multiples of 32 for %s", MetaTensorShape, TaskOCR)
	}
	for _, key := range []string{MetaSourceWidth, MetaSourceHeight} {
		if strings.TrimSpace(req.Meta[key]) == "" {
			return fmt.Errorf("missing %s", key)
		}
	}
	if requireLetterbox {
		for _, key := range []string{MetaLetterboxScale, MetaLetterboxPadX, MetaLetterboxPadY} {
			if strings.TrimSpace(req.Meta[key]) == "" {
				return fmt.Errorf("missing %s", key)
			}
		}
	}
	return nil
}

func validateRawImageMIME(mime string) error {
	if mimetype.EqualsAny(mime, SupportedImageMimeTypes...) {
		return nil
	}
	return fmt.Errorf("unsupported image payload_mime: %s", mime)
}

func looksLikeJSONPixelPayload(payload []byte) bool {
	trimmed := strings.TrimSpace(string(payload))
	return strings.HasPrefix(trimmed, "[") || strings.Contains(trimmed, "\"pixels\"")
}
