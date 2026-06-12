package types_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"math"
	"testing"

	"github.com/edwinzhancn/lumen-sdk/pkg/types"
)

const siglipPreprocessID = "siglip2_base_patch16_224_image_v1"

func siglipPreprocessor(t *testing.T) types.TensorPreprocessor {
	t.Helper()
	preprocessor, ok := types.DefaultTensorPreprocessorRegistry().Lookup(siglipPreprocessID)
	if !ok {
		t.Fatalf("preprocessor %s not registered", siglipPreprocessID)
	}
	return preprocessor
}

func solidHWC(width, height int, r, g, b byte) []byte {
	data := make([]byte, 0, width*height*3)
	for i := 0; i < width*height; i++ {
		data = append(data, r, g, b)
	}
	return data
}

func solidPNG(t *testing.T, width, height int, c color.NRGBA) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetNRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func firstTensorValue(t *testing.T, payload []byte) float32 {
	t.Helper()
	if len(payload) < 4 {
		t.Fatalf("tensor payload too short: %d bytes", len(payload))
	}
	return math.Float32frombits(binary.LittleEndian.Uint32(payload[:4]))
}

// Matching decoded input must win over encoded input: the caller's pipeline
// already produced model-sized pixels, so no in-process decode should happen.
func TestPreprocessPrefersMatchingDecodedInput(t *testing.T) {
	preprocessor := siglipPreprocessor(t)

	out, err := preprocessor.Preprocess(context.Background(), types.ImageInput{
		Encoded:     solidPNG(t, 224, 224, color.NRGBA{R: 0, G: 0, B: 0, A: 255}),
		PayloadMIME: "image/png",
		Data:        solidHWC(224, 224, 255, 255, 255),
		Width:       224,
		Height:      224,
		Channels:    3,
		Layout:      "HWC",
		DType:       "uint8",
		ColorSpace:  "RGB",
	})
	if err != nil {
		t.Fatalf("preprocess: %v", err)
	}

	// White pixels normalize to ~+1 with SigLIP mean/std 0.5/0.5; the black
	// encoded image would normalize to ~-1.
	if got := firstTensorValue(t, out.Payload); math.Abs(float64(got-1.0)) > 1e-3 {
		t.Fatalf("expected decoded (white) input to win, first value = %v", got)
	}
}

// Decoded input with the wrong shape must degrade to the encoded path instead
// of failing, because callers pass both and Hub owns the contract.
func TestPreprocessFallsBackToEncodedOnShapeMismatch(t *testing.T) {
	preprocessor := siglipPreprocessor(t)

	out, err := preprocessor.Preprocess(context.Background(), types.ImageInput{
		Encoded:     solidPNG(t, 64, 64, color.NRGBA{R: 0, G: 0, B: 0, A: 255}),
		PayloadMIME: "image/png",
		Data:        solidHWC(100, 100, 255, 255, 255),
		Width:       100,
		Height:      100,
		Channels:    3,
		Layout:      "HWC",
		DType:       "uint8",
		ColorSpace:  "RGB",
	})
	if err != nil {
		t.Fatalf("preprocess: %v", err)
	}

	if got := firstTensorValue(t, out.Payload); math.Abs(float64(got+1.0)) > 1e-3 {
		t.Fatalf("expected encoded (black) fallback, first value = %v", got)
	}
}

func TestPreprocessDecodedOnlyShapeMismatchFails(t *testing.T) {
	preprocessor := siglipPreprocessor(t)

	_, err := preprocessor.Preprocess(context.Background(), types.ImageInput{
		Data:       solidHWC(100, 100, 255, 255, 255),
		Width:      100,
		Height:     100,
		Channels:   3,
		Layout:     "HWC",
		DType:      "uint8",
		ColorSpace: "RGB",
	})
	if err == nil {
		t.Fatal("expected error for mismatched decoded-only input")
	}
}

func TestPreprocessEmptyInputFails(t *testing.T) {
	preprocessor := siglipPreprocessor(t)

	_, err := preprocessor.Preprocess(context.Background(), types.ImageInput{})
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}
