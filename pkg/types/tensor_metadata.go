package types

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	pb "github.com/edwinzhancn/lumen-sdk/proto"
)

const (
	// Reserved metadata keys for Lumen tensor fast-path requests and responses.
	MetaInputKind             = "lumen.input.kind"
	MetaTensorDType           = "lumen.tensor.dtype"
	MetaTensorShape           = "lumen.tensor.shape"
	MetaTensorLayout          = "lumen.tensor.layout"
	MetaTensorFormat          = "lumen.tensor.format"
	MetaTensorByteOrder       = "lumen.tensor.byte_order"
	MetaPreprocessID          = "lumen.preprocess.id"
	MetaPreprocessSkip        = "lumen.preprocess.skip"
	MetaModelID               = "lumen.model.id"
	MetaModelVersion          = "lumen.model.version"
	MetaOutputKind            = "lumen.output.kind"
	MetaOutputTensorDType     = "lumen.output.tensor.dtype"
	MetaOutputTensorShape     = "lumen.output.tensor.shape"
	MetaOutputTensorLayout    = "lumen.output.tensor.layout"
	MetaOutputTensorFormat    = "lumen.output.tensor.format"
	MetaOutputTensorByteOrder = "lumen.output.tensor.byte_order"
	InputKindRaw              = "raw"
	InputKindTensor           = "tensor"
	OutputKindRaw             = "raw"
	OutputKindTensor          = "tensor"
	TensorFormatContig        = "contiguous"
	TensorByteOrderLittle     = "little"
	DefaultTensorMIME         = "application/octet-stream"
)

// TensorDescriptor describes a model-ready tensor carried through InferRequest.Meta.
// It intentionally mirrors the v1 tensor fast-path metadata contract without
// changing the protobuf wire shape.
type TensorDescriptor struct {
	DType          string
	Shape          []int64
	Layout         string
	Format         string
	ByteOrder      string
	PreprocessID   string
	PreprocessSkip bool
	ModelID        string
	ModelVersion   string
}

// TensorValidationOptions lets task backends tighten the generic tensor contract
// with task-specific allowlists. Empty allowlists use the SDK defaults, except
// AllowedPreprocessIDs: when empty, any non-empty preprocess ID is accepted.
type TensorValidationOptions struct {
	AllowedDTypes        []string
	AllowedLayouts       []string
	AllowedFormats       []string
	AllowedByteOrders    []string
	AllowedPreprocessIDs []string

	// DisableSingleBatchCheck permits tensor payloads whose leading batch
	// dimension is greater than one. It is false by default for v1.
	DisableSingleBatchCheck bool
}

// TensorOutputValidationOptions lets callers tighten tensor response validation.
// Empty allowlists use the SDK defaults.
type TensorOutputValidationOptions struct {
	AllowedDTypes     []string
	AllowedLayouts    []string
	AllowedFormats    []string
	AllowedByteOrders []string

	// DisableSingleBatchCheck permits tensor outputs whose leading batch
	// dimension is greater than one. It is false by default for v1.
	DisableSingleBatchCheck bool
}

// TensorResponse is a validated model-ready tensor returned by an InferResponse.
type TensorResponse struct {
	Descriptor *TensorDescriptor
	Data       []byte
	ResultMime string
	Meta       map[string]string
}

// ValidateTensorFastPath validates tensor fast-path metadata when present.
// Raw requests return (nil, nil); tensor requests return their parsed descriptor.
func ValidateTensorFastPath(req *pb.InferRequest, opts TensorValidationOptions) (*TensorDescriptor, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	desc, isTensor, err := ParseTensorDescriptor(req.Meta)
	if err != nil {
		return nil, err
	}
	if !isTensor {
		return nil, nil
	}

	if err := validateTensorDescriptor(desc, req.Payload, opts); err != nil {
		return nil, err
	}
	return desc, nil
}

// ValidateTensorResponse validates tensor output metadata when present.
// Raw and legacy JSON responses return (nil, nil); tensor responses return their descriptor.
func ValidateTensorResponse(resp *pb.InferResponse, opts TensorOutputValidationOptions) (*TensorDescriptor, error) {
	if resp == nil {
		return nil, fmt.Errorf("response cannot be nil")
	}

	desc, isTensor, err := ParseOutputTensorDescriptor(resp.Meta)
	if err != nil {
		return nil, err
	}
	if !isTensor {
		return nil, nil
	}

	if strings.TrimSpace(resp.ResultMime) != DefaultTensorMIME {
		return nil, fmt.Errorf("tensor response result_mime must be %s, got %s", DefaultTensorMIME, resp.ResultMime)
	}

	if err := validateTensorPayload(desc, resp.Result, tensorValidationCoreOptions{
		AllowedDTypes:             opts.AllowedDTypes,
		AllowedLayouts:            opts.AllowedLayouts,
		AllowedFormats:            opts.AllowedFormats,
		AllowedByteOrders:         opts.AllowedByteOrders,
		DTypeKey:                  MetaOutputTensorDType,
		ShapeKey:                  MetaOutputTensorShape,
		LayoutKey:                 MetaOutputTensorLayout,
		FormatKey:                 MetaOutputTensorFormat,
		ByteOrderKey:              MetaOutputTensorByteOrder,
		DisableSingleBatchCheck:   opts.DisableSingleBatchCheck,
		RequirePreprocessContract: false,
	}); err != nil {
		return nil, err
	}
	return desc, nil
}

// ParseTensorDescriptor parses InferRequest.Meta into a tensor descriptor.
// The bool return value is true only for lumen.input.kind=tensor.
func ParseTensorDescriptor(meta map[string]string) (*TensorDescriptor, bool, error) {
	kind := strings.ToLower(strings.TrimSpace(meta[MetaInputKind]))
	if kind == "" || kind == InputKindRaw {
		return nil, false, nil
	}
	if kind != InputKindTensor {
		return nil, false, fmt.Errorf("unsupported %s %q", MetaInputKind, meta[MetaInputKind])
	}

	desc := &TensorDescriptor{
		DType:          normalizeTensorDType(meta[MetaTensorDType]),
		Layout:         normalizeTensorLayout(meta[MetaTensorLayout]),
		Format:         normalizeTensorFormat(meta[MetaTensorFormat]),
		ByteOrder:      normalizeTensorByteOrder(meta[MetaTensorByteOrder]),
		PreprocessID:   strings.TrimSpace(meta[MetaPreprocessID]),
		PreprocessSkip: boolMetaIsTrue(meta[MetaPreprocessSkip]),
		ModelID:        strings.TrimSpace(meta[MetaModelID]),
		ModelVersion:   strings.TrimSpace(meta[MetaModelVersion]),
	}

	shapeValue := strings.TrimSpace(meta[MetaTensorShape])
	if shapeValue != "" {
		if err := json.Unmarshal([]byte(shapeValue), &desc.Shape); err != nil {
			return nil, false, fmt.Errorf("invalid %s: %w", MetaTensorShape, err)
		}
	}

	return desc, true, nil
}

// ParseOutputTensorDescriptor parses InferResponse.Meta into an output tensor descriptor.
// The bool return value is true only for lumen.output.kind=tensor.
func ParseOutputTensorDescriptor(meta map[string]string) (*TensorDescriptor, bool, error) {
	kind := strings.ToLower(strings.TrimSpace(meta[MetaOutputKind]))
	if kind == "" || kind == OutputKindRaw {
		return nil, false, nil
	}
	if kind != OutputKindTensor {
		return nil, false, fmt.Errorf("unsupported %s %q", MetaOutputKind, meta[MetaOutputKind])
	}

	desc := &TensorDescriptor{
		DType:        normalizeTensorDType(meta[MetaOutputTensorDType]),
		Layout:       normalizeTensorLayout(meta[MetaOutputTensorLayout]),
		Format:       normalizeTensorFormat(meta[MetaOutputTensorFormat]),
		ByteOrder:    normalizeTensorByteOrder(meta[MetaOutputTensorByteOrder]),
		ModelID:      strings.TrimSpace(meta[MetaModelID]),
		ModelVersion: strings.TrimSpace(meta[MetaModelVersion]),
	}

	shapeValue := strings.TrimSpace(meta[MetaOutputTensorShape])
	if shapeValue != "" {
		if err := json.Unmarshal([]byte(shapeValue), &desc.Shape); err != nil {
			return nil, false, fmt.Errorf("invalid %s: %w", MetaOutputTensorShape, err)
		}
	}

	return desc, true, nil
}

func validateTensorDescriptor(desc *TensorDescriptor, payload []byte, opts TensorValidationOptions) error {
	return validateTensorPayload(desc, payload, tensorValidationCoreOptions{
		AllowedDTypes:             opts.AllowedDTypes,
		AllowedLayouts:            opts.AllowedLayouts,
		AllowedFormats:            opts.AllowedFormats,
		AllowedByteOrders:         opts.AllowedByteOrders,
		AllowedPreprocessIDs:      opts.AllowedPreprocessIDs,
		DTypeKey:                  MetaTensorDType,
		ShapeKey:                  MetaTensorShape,
		LayoutKey:                 MetaTensorLayout,
		FormatKey:                 MetaTensorFormat,
		ByteOrderKey:              MetaTensorByteOrder,
		DisableSingleBatchCheck:   opts.DisableSingleBatchCheck,
		RequirePreprocessContract: true,
	})
}

type tensorValidationCoreOptions struct {
	AllowedDTypes             []string
	AllowedLayouts            []string
	AllowedFormats            []string
	AllowedByteOrders         []string
	AllowedPreprocessIDs      []string
	DTypeKey                  string
	ShapeKey                  string
	LayoutKey                 string
	FormatKey                 string
	ByteOrderKey              string
	DisableSingleBatchCheck   bool
	RequirePreprocessContract bool
}

func withDefaultTensorValidationKeys(opts tensorValidationCoreOptions) tensorValidationCoreOptions {
	if opts.DTypeKey == "" {
		opts.DTypeKey = MetaTensorDType
	}
	if opts.ShapeKey == "" {
		opts.ShapeKey = MetaTensorShape
	}
	if opts.LayoutKey == "" {
		opts.LayoutKey = MetaTensorLayout
	}
	if opts.FormatKey == "" {
		opts.FormatKey = MetaTensorFormat
	}
	if opts.ByteOrderKey == "" {
		opts.ByteOrderKey = MetaTensorByteOrder
	}
	return opts
}

func validateTensorPayload(desc *TensorDescriptor, payload []byte, opts tensorValidationCoreOptions) error {
	opts = withDefaultTensorValidationKeys(opts)
	if desc.DType == "" {
		return fmt.Errorf("missing %s", opts.DTypeKey)
	}
	if desc.Layout == "" {
		return fmt.Errorf("missing %s", opts.LayoutKey)
	}
	if desc.Format == "" {
		return fmt.Errorf("missing %s", opts.FormatKey)
	}
	if desc.ByteOrder == "" {
		return fmt.Errorf("missing %s", opts.ByteOrderKey)
	}
	if len(desc.Shape) == 0 {
		return fmt.Errorf("missing %s", opts.ShapeKey)
	}
	if opts.RequirePreprocessContract {
		if desc.PreprocessID == "" {
			return fmt.Errorf("missing %s", MetaPreprocessID)
		}
		if !desc.PreprocessSkip {
			return fmt.Errorf("%s must be true for tensor fast-path requests", MetaPreprocessSkip)
		}
	}

	if !containsNormalized(opts.AllowedDTypes, defaultTensorDTypes(), desc.DType, normalizeTensorDType) {
		return fmt.Errorf("unsupported %s %q", opts.DTypeKey, desc.DType)
	}
	if !containsNormalized(opts.AllowedLayouts, defaultTensorLayouts(), desc.Layout, normalizeTensorLayout) {
		return fmt.Errorf("unsupported %s %q", opts.LayoutKey, desc.Layout)
	}
	if !containsNormalized(opts.AllowedFormats, []string{TensorFormatContig}, desc.Format, normalizeTensorFormat) {
		return fmt.Errorf("unsupported %s %q", opts.FormatKey, desc.Format)
	}
	if !containsNormalized(opts.AllowedByteOrders, []string{TensorByteOrderLittle}, desc.ByteOrder, normalizeTensorByteOrder) {
		return fmt.Errorf("unsupported %s %q", opts.ByteOrderKey, desc.ByteOrder)
	}
	if opts.RequirePreprocessContract && len(opts.AllowedPreprocessIDs) > 0 && !containsExact(opts.AllowedPreprocessIDs, desc.PreprocessID) {
		return fmt.Errorf("unsupported %s %q", MetaPreprocessID, desc.PreprocessID)
	}

	elementCount, err := tensorElementCount(desc.Shape)
	if err != nil {
		return err
	}
	elementSize, ok := TensorElementSize(desc.DType)
	if !ok {
		return fmt.Errorf("unsupported %s %q", opts.DTypeKey, desc.DType)
	}
	if elementCount > math.MaxInt64/int64(elementSize) {
		return fmt.Errorf("tensor byte length overflows int64")
	}
	expectedBytes := elementCount * int64(elementSize)
	if int64(len(payload)) != expectedBytes {
		return fmt.Errorf("tensor payload length mismatch: expected %d bytes, got %d", expectedBytes, len(payload))
	}

	if !opts.DisableSingleBatchCheck && strings.HasPrefix(desc.Layout, "N") && len(desc.Shape) > 0 && desc.Shape[0] != 1 {
		return fmt.Errorf("tensor payload batching is not supported in v1: leading batch dimension must be 1, got %d", desc.Shape[0])
	}

	return nil
}

func tensorElementCount(shape []int64) (int64, error) {
	count := int64(1)
	for i, dim := range shape {
		if dim <= 0 {
			return 0, fmt.Errorf("invalid %s dimension at index %d: %d", MetaTensorShape, i, dim)
		}
		if count > math.MaxInt64/dim {
			return 0, fmt.Errorf("tensor element count overflows int64")
		}
		count *= dim
	}
	return count, nil
}

// TensorElementSize returns the byte size for supported fast-path tensor dtypes.
func TensorElementSize(dtype string) (int, bool) {
	switch normalizeTensorDType(dtype) {
	case "fp32":
		return 4, true
	case "fp16":
		return 2, true
	case "uint8":
		return 1, true
	case "int64":
		return 8, true
	default:
		return 0, false
	}
}

func defaultTensorDTypes() []string {
	return []string{"fp32", "fp16", "uint8", "int64"}
}

func defaultTensorLayouts() []string {
	return []string{"NCHW", "NHWC", "CHW"}
}

func containsNormalized(values, defaults []string, target string, normalize func(string) string) bool {
	if len(values) == 0 {
		values = defaults
	}
	for _, value := range values {
		if normalize(value) == target {
			return true
		}
	}
	return false
}

func containsExact(values []string, target string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == target {
			return true
		}
	}
	return false
}

func normalizeTensorDType(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeTensorLayout(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func normalizeTensorFormat(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeTensorByteOrder(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func boolMetaIsTrue(value string) bool {
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	return err == nil && parsed
}
