package types

// SupportedImageMimeTypes lists the image MIME types accepted by Lumen ML services.
//
// These formats are supported for image-based operations including:
//   - Image embedding generation
//   - Image classification
//   - Face detection and recognition
//
// Role in project: Defines the contract between clients and ML nodes for image
// data. Used for validation in request builders (NewClassificationRequest, etc.).
//
// Example:
//
//	imageData, _ := os.ReadFile("photo.jpg")
//	mime := mimetype.Detect(imageData).String()
//	if !mimetype.EqualsAny(mime, types.SupportedImageMimeTypes...) {
//	    log.Fatal("Unsupported image format")
//	}
var SupportedImageMimeTypes = []string{
	"image/jpeg",
	"image/png",
	"image/webp",
}

// SupportedTextMimeTypes lists the text MIME types accepted by Lumen ML services.
//
// These formats are supported for text-based operations including:
//   - Text embedding generation
//   - Semantic search
//   - Text analysis tasks
//
// Role in project: Defines acceptable text formats for ML operations. Used for
// validation in NewEmbeddingRequest and other text-processing functions.
//
// Example:
//
//	textData := []byte("Machine learning is transforming AI")
//	embReq, err := types.NewEmbeddingRequest(textData)
//	// Automatically validates against SupportedTextMimeTypes
var SupportedTextMimeTypes = []string{
	"text/plain",
	"text/markdown",
	"text/html",
}
