package types

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"strings"
	"sync"

	"github.com/disintegration/imaging"
	_ "golang.org/x/image/webp"
)

// ImageInput is the SDK-owned preprocessor input shape. Callers can provide an
// encoded image, an already-decoded HWC RGB uint8 image, or both. Decoded input
// matching the preprocessor's target dimensions is preferred: it skips the
// in-process decode/resize entirely (the caller's image pipeline, e.g. libvips,
// already produced model-sized pixels). Encoded input is the fallback whenever
// decoded input is absent or does not match the expected shape.
type ImageInput struct {
	Encoded     []byte
	PayloadMIME string

	Data       []byte
	Width      int
	Height     int
	Channels   int
	Layout     string
	DType      string
	ColorSpace string
}

// TensorPayload is a model-ready tensor payload plus the metadata needed to send
// it through InferRequest.
type TensorPayload struct {
	Payload     []byte
	PayloadMIME string
	Descriptor  TensorDescriptor
}

type TensorPreprocessor interface {
	ID() string
	Preprocess(ctx context.Context, input ImageInput) (*TensorPayload, error)
}

type TensorPreprocessorRegistry struct {
	mu            sync.RWMutex
	preprocessors map[string]TensorPreprocessor
}

func NewTensorPreprocessorRegistry(preprocessors ...TensorPreprocessor) *TensorPreprocessorRegistry {
	registry := &TensorPreprocessorRegistry{preprocessors: make(map[string]TensorPreprocessor)}
	for _, preprocessor := range preprocessors {
		registry.Register(preprocessor)
	}
	return registry
}

func (r *TensorPreprocessorRegistry) Register(preprocessor TensorPreprocessor) {
	if r == nil || preprocessor == nil || strings.TrimSpace(preprocessor.ID()) == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.preprocessors[preprocessor.ID()] = preprocessor
}

func (r *TensorPreprocessorRegistry) Lookup(id string) (TensorPreprocessor, bool) {
	if r == nil {
		return nil, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	preprocessor, ok := r.preprocessors[strings.TrimSpace(id)]
	return preprocessor, ok
}

var builtinTensorPreprocessors = NewTensorPreprocessorRegistry(
	newImageTensorPreprocessor(PreprocessSigLIP2BasePatch16_224Image, 224, 224, false, imaging.Linear, [3]float32{0.5, 0.5, 0.5}, [3]float32{0.5, 0.5, 0.5}),
	newImageTensorPreprocessor(PreprocessSigLIP2SO400MPatch14_384Image, 384, 384, false, imaging.Linear, [3]float32{0.5, 0.5, 0.5}, [3]float32{0.5, 0.5, 0.5}),
	newImageTensorPreprocessor(PreprocessBioCLIP224Image, 224, 224, true, imaging.CatmullRom, [3]float32{0.48145466, 0.4578275, 0.40821073}, [3]float32{0.26862954, 0.26130258, 0.27577711}),
)

func DefaultTensorPreprocessorRegistry() *TensorPreprocessorRegistry {
	return builtinTensorPreprocessors
}

type imageTensorPreprocessor struct {
	id         string
	width      int
	height     int
	centerCrop bool
	filter     imaging.ResampleFilter
	mean       [3]float32
	std        [3]float32
}

func newImageTensorPreprocessor(id string, width, height int, centerCrop bool, filter imaging.ResampleFilter, mean, std [3]float32) TensorPreprocessor {
	return &imageTensorPreprocessor{id: id, width: width, height: height, centerCrop: centerCrop, filter: filter, mean: mean, std: std}
}

func (p *imageTensorPreprocessor) ID() string { return p.id }

func (p *imageTensorPreprocessor) Preprocess(ctx context.Context, input ImageInput) (*TensorPayload, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var rgb []byte
	if len(input.Data) > 0 {
		decoded, err := p.hwcRGBInput(input)
		if err == nil {
			rgb = decoded
		} else if len(input.Encoded) == 0 {
			return nil, err
		}
		// Decoded input that does not match the target shape falls through to
		// the encoded path so callers with both inputs degrade gracefully.
	}
	if rgb == nil {
		if len(input.Encoded) == 0 {
			return nil, fmt.Errorf("tensor preprocessor %s requires encoded or decoded image input", p.id)
		}
		decoded, err := decodeImage(input.Encoded)
		if err != nil {
			return nil, err
		}
		rgb = imageToRGB(p.prepareImage(decoded))
	}

	payload := p.rgbToNCHWFP32LE(rgb)
	return &TensorPayload{
		Payload:     payload,
		PayloadMIME: DefaultTensorMIME,
		Descriptor: TensorDescriptor{
			DType:          "fp32",
			Shape:          []int64{1, 3, int64(p.height), int64(p.width)},
			Layout:         "NCHW",
			Format:         TensorFormatContig,
			ByteOrder:      TensorByteOrderLittle,
			PreprocessID:   p.id,
			PreprocessSkip: true,
		},
	}, nil
}

func decodeImage(encoded []byte) (image.Image, error) {
	img, _, err := image.Decode(bytes.NewReader(encoded))
	if err != nil {
		return nil, fmt.Errorf("decode image for tensor preprocessing: %w", err)
	}
	return img, nil
}

func (p *imageTensorPreprocessor) prepareImage(img image.Image) image.Image {
	if p.centerCrop {
		// CLIP-style: resize shortest edge to the target, then center crop.
		bounds := img.Bounds()
		width := bounds.Dx()
		height := bounds.Dy()
		shortest := width
		if height < shortest {
			shortest = height
		}
		if shortest > 0 && shortest != p.width {
			scale := float64(p.width) / float64(shortest)
			width = maxInt(1, int(float64(width)*scale+0.5))
			height = maxInt(1, int(float64(height)*scale+0.5))
			img = resizeImage(img, width, height, p.filter)
		}
		return centerCropImage(img, p.width, p.height, p.filter)
	}

	// SigLIP-style (`do_center_crop=false`): a single direct resize to the
	// target size, matching HF training-time preprocessing. A shortest-edge
	// pre-pass would add a second resampling pass the model never saw.
	if img.Bounds().Dx() != p.width || img.Bounds().Dy() != p.height {
		return resizeImage(img, p.width, p.height, p.filter)
	}
	return img
}

func (p *imageTensorPreprocessor) hwcRGBInput(input ImageInput) ([]byte, error) {
	if input.Width != p.width || input.Height != p.height || input.Channels != 3 {
		return nil, fmt.Errorf("decoded tensor preprocessor input must be %dx%dx3, got %dx%dx%d", p.width, p.height, input.Width, input.Height, input.Channels)
	}
	if !strings.EqualFold(input.Layout, "HWC") || !strings.EqualFold(input.DType, "uint8") || !strings.EqualFold(input.ColorSpace, "RGB") {
		return nil, fmt.Errorf("decoded tensor preprocessor input must be HWC uint8 RGB")
	}
	expected := p.width * p.height * 3
	if len(input.Data) != expected {
		return nil, fmt.Errorf("decoded tensor preprocessor input has %d bytes, want %d", len(input.Data), expected)
	}
	return append([]byte(nil), input.Data...), nil
}

func toRGBImage(img image.Image) *image.NRGBA {
	bounds := img.Bounds()
	rgb := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	for y := 0; y < bounds.Dy(); y++ {
		for x := 0; x < bounds.Dx(); x++ {
			r, g, b, _ := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			offset := rgb.PixOffset(x, y)
			rgb.Pix[offset+0] = byte(r >> 8)
			rgb.Pix[offset+1] = byte(g >> 8)
			rgb.Pix[offset+2] = byte(b >> 8)
			rgb.Pix[offset+3] = 0xff
		}
	}
	return rgb
}

func resizeImage(img image.Image, width, height int, filter imaging.ResampleFilter) *image.NRGBA {
	return imaging.Resize(img, width, height, filter)
}

func centerCropImage(img image.Image, width, height int, filter imaging.ResampleFilter) image.Image {
	bounds := img.Bounds()
	if bounds.Dx() < width || bounds.Dy() < height {
		img = resizeImage(img, width, height, filter)
		bounds = img.Bounds()
	}
	x0 := bounds.Min.X + (bounds.Dx()-width)/2
	y0 := bounds.Min.Y + (bounds.Dy()-height)/2
	return imaging.Crop(img, image.Rect(x0, y0, x0+width, y0+height))
}

func imageToRGB(img image.Image) []byte {
	if nrgba, ok := img.(*image.NRGBA); ok {
		return nrgbaToRGB(nrgba)
	}
	return nrgbaToRGB(toRGBImage(img))
}

func nrgbaToRGB(img *image.NRGBA) []byte {
	width := img.Bounds().Dx()
	height := img.Bounds().Dy()
	out := make([]byte, 0, width*height*3)
	for y := 0; y < height; y++ {
		row := img.Pix[y*img.Stride : y*img.Stride+width*4]
		for x := 0; x < width*4; x += 4 {
			out = append(out, row[x], row[x+1], row[x+2])
		}
	}
	return out
}

func (p *imageTensorPreprocessor) rgbToNCHWFP32LE(rgb []byte) []byte {
	plane := p.width * p.height
	payload := make([]byte, plane*3*4)
	for y := 0; y < p.height; y++ {
		for x := 0; x < p.width; x++ {
			pixelOffset := (y*p.width + x) * 3
			for c := 0; c < 3; c++ {
				value := float32(rgb[pixelOffset+c]) * (1.0 / 255.0)
				value = (value - p.mean[c]) / p.std[c]
				outOffset := (c*plane + y*p.width + x) * 4
				binary.LittleEndian.PutUint32(payload[outOffset:outOffset+4], math.Float32bits(value))
			}
		}
	}
	return payload
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
