package types_test

import (
	"strings"
	"testing"

	"github.com/edwinzhancn/lumen-sdk/pkg/types"
)

func TestSupportedImageMimeTypesConstant(t *testing.T) {
	expected := []string{"image/jpeg", "image/png", "image/webp"}
	
	mimeTypes := strings.Split(types.SupportedImageMimeTypes, ",")
	
	if len(mimeTypes) != len(expected) {
		t.Errorf("Expected %d MIME types, got %d", len(expected), len(mimeTypes))
	}
	
	for _, expected := range expected {
		found := false
		for _, mimeType := range mimeTypes {
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
	
	mimeTypes := strings.Split(types.SupportedTextMimeTypes, ",")
	
	if len(mimeTypes) != len(expected) {
		t.Errorf("Expected %d MIME types, got %d", len(expected), len(mimeTypes))
	}
	
	for _, expected := range expected {
		found := false
		for _, mimeType := range mimeTypes {
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
	if !strings.Contains(types.SupportedImageMimeTypes, "image/jpeg") {
		t.Error("Expected SupportedImageMimeTypes to contain 'image/jpeg'")
	}
}

func TestSupportedImageMimeTypesContainsPNG(t *testing.T) {
	if !strings.Contains(types.SupportedImageMimeTypes, "image/png") {
		t.Error("Expected SupportedImageMimeTypes to contain 'image/png'")
	}
}

func TestSupportedImageMimeTypesContainsWebP(t *testing.T) {
	if !strings.Contains(types.SupportedImageMimeTypes, "image/webp") {
		t.Error("Expected SupportedImageMimeTypes to contain 'image/webp'")
	}
}

func TestSupportedTextMimeTypesContainsPlainText(t *testing.T) {
	if !strings.Contains(types.SupportedTextMimeTypes, "text/plain") {
		t.Error("Expected SupportedTextMimeTypes to contain 'text/plain'")
	}
}

func TestSupportedTextMimeTypesContainsMarkdown(t *testing.T) {
	if !strings.Contains(types.SupportedTextMimeTypes, "text/markdown") {
		t.Error("Expected SupportedTextMimeTypes to contain 'text/markdown'")
	}
}

func TestSupportedTextMimeTypesContainsHTML(t *testing.T) {
	if !strings.Contains(types.SupportedTextMimeTypes, "text/html") {
		t.Error("Expected SupportedTextMimeTypes to contain 'text/html'")
	}
}
