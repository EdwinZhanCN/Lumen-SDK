package types_test

import (
	"testing"

	"github.com/edwinzhancn/lumen-sdk/pkg/types"
)

func TestSupportedImageMimeTypesConstant(t *testing.T) {
	expected := []string{"image/jpeg", "image/png", "image/webp"}

	if len(types.SupportedImageMimeTypes) != len(expected) {
		t.Errorf("Expected %d MIME types, got %d", len(expected), len(types.SupportedImageMimeTypes))
	}

	for _, expected := range expected {
		found := false
		for _, mimeType := range types.SupportedImageMimeTypes {
			if mimeType == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected MIME type '%s' not found in SupportedImageMimeTypes", expected)
		}
	}
}

func TestSupportedTextMimeTypesConstant(t *testing.T) {
	expected := []string{"text/plain", "text/markdown", "text/html"}

	if len(types.SupportedTextMimeTypes) != len(expected) {
		t.Errorf("Expected %d MIME types, got %d", len(expected), len(types.SupportedTextMimeTypes))
	}

	for _, expected := range expected {
		found := false
		for _, mimeType := range types.SupportedTextMimeTypes {
			if mimeType == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected MIME type '%s' not found in SupportedTextMimeTypes", expected)
		}
	}
}

func TestSupportedImageMimeTypesContainsJPEG(t *testing.T) {
	contains := false
	for _, mimeType := range types.SupportedImageMimeTypes {
		if mimeType == "image/jpeg" {
			contains = true
			break
		}
	}
	if !contains {
		t.Error("Expected SupportedImageMimeTypes to contain 'image/jpeg'")
	}
}

func TestSupportedImageMimeTypesContainsPNG(t *testing.T) {
	contains := false
	for _, mimeType := range types.SupportedImageMimeTypes {
		if mimeType == "image/png" {
			contains = true
			break
		}
	}
	if !contains {
		t.Error("Expected SupportedImageMimeTypes to contain 'image/png'")
	}
}

func TestSupportedImageMimeTypesContainsWebP(t *testing.T) {
	contains := false
	for _, mimeType := range types.SupportedImageMimeTypes {
		if mimeType == "image/webp" {
			contains = true
			break
		}
	}
	if !contains {
		t.Error("Expected SupportedImageMimeTypes to contain 'image/webp'")
	}
}

func TestSupportedTextMimeTypesContainsPlainText(t *testing.T) {
	contains := false
	for _, mimeType := range types.SupportedTextMimeTypes {
		if mimeType == "text/plain" {
			contains = true
			break
		}
	}
	if !contains {
		t.Error("Expected SupportedTextMimeTypes to contain 'text/plain'")
	}
}

func TestSupportedTextMimeTypesContainsMarkdown(t *testing.T) {
	contains := false
	for _, mimeType := range types.SupportedTextMimeTypes {
		if mimeType == "text/markdown" {
			contains = true
			break
		}
	}
	if !contains {
		t.Error("Expected SupportedTextMimeTypes to contain 'text/markdown'")
	}
}

func TestSupportedTextMimeTypesContainsHTML(t *testing.T) {
	contains := false
	for _, mimeType := range types.SupportedTextMimeTypes {
		if mimeType == "text/html" {
			contains = true
			break
		}
	}
	if !contains {
		t.Error("Expected SupportedTextMimeTypes to contain 'text/html'")
	}
}
