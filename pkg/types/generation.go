package types

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/gabriel-vasile/mimetype"
)

// TextGenerationV1 represents a text generation response from Lumen VLM services.
//
// This structure contains the generated text along with metadata about the generation
// process, including token counts, completion reasons, and optional generation parameters.
//
// Role in project: Output structure for text generation tasks. Used in chat responses,
// text completion, summarization, and other natural language generation scenarios.
type TextGenerationV1 struct {
	Text            string                  `json:"text"`
	FinishReason    string                  `json:"finish_reason"`
	GeneratedTokens int                     `json:"generated_tokens"`
	InputTokens     int                     `json:"input_tokens,omitempty"`
	ModelID         string                  `json:"model_id"`
	Metadata        *TextGenerationMetadata `json:"metadata,omitempty"`
}

// TextGenerationMetadata contains optional metadata about the text generation process.
//
// This structure captures generation parameters and performance metrics that can be
// useful for debugging, optimization, and understanding the generation behavior.
type TextGenerationMetadata struct {
	Temperature      float64 `json:"temperature,omitempty"`
	TopP             float64 `json:"top_p,omitempty"`
	MaxTokens        int     `json:"max_tokens,omitempty"`
	Seed             int64   `json:"seed,omitempty"`
	GenerationTimeMs float64 `json:"generation_time_ms,omitempty"`
	StreamingChunks  int     `json:"streaming_chunks,omitempty"`
}

// ImageTextGenerationRequest represents a request for image+text generation (VLM).
//
// This structure encapsulates an image payload, prompt or messages, and generation parameters
// for vision-language model tasks.
//
// Role in project: Input structure for VLM text generation tasks.
//
// Example:
//
//	imageData, _ := os.ReadFile("cat.jpg")
//	req, err := types.NewImageTextGenerationRequest(imageData, "image/jpeg").
//		WithMaxTokens(512).
//		WithTemperature(0.0)
//
//	inferReq := types.NewInferRequest("vlm").
//	    ForImageTextGeneration(req, "fastvlm-2b-onnx").
//	    Build()
type ImageTextGenerationRequest struct {
	Payload     []byte            `json:"payload"`
	PayloadMime string            `json:"payload_mime"`
	Meta        map[string]string `json:"meta"`
}

type ImageTextGenerationRequestOption func(*ImageTextGenerationRequest)

// WithMaxTokens sets the maximum number of new tokens to generate.
func WithMaxTokens(maxTokens int) ImageTextGenerationRequestOption {
	return func(req *ImageTextGenerationRequest) {
		req.Meta["max_new_tokens"] = fmt.Sprintf("%d", maxTokens)
	}
}

// WithTemperature sets the sampling temperature for the generation request.
// Higher values (e.g., 0.8) make output more random, while lower values (e.g., 0.2) make it more deterministic.
func WithTemperature(temperature float64) ImageTextGenerationRequestOption {
	return func(req *ImageTextGenerationRequest) {
		req.Meta["temperature"] = fmt.Sprintf("%.1f", temperature)
	}
}

// WithTopP sets the nucleus sampling parameter for the generation request.
// Controls diversity via probability threshold: 0.9 means considering the top 90% probability mass.
func WithTopP(topP float64) ImageTextGenerationRequestOption {
	return func(req *ImageTextGenerationRequest) {
		req.Meta["top_p"] = fmt.Sprintf("%.1f", topP)
	}
}

// WithRepetitionPenalty sets the repetition penalty to discourage repetitive text.
func WithRepetitionPenalty(penalty float64) ImageTextGenerationRequestOption {
	return func(req *ImageTextGenerationRequest) {
		req.Meta["repetition_penalty"] = fmt.Sprintf("%.1f", penalty)
	}
}

// WithDoSample enables or disables sampling for token generation.
func WithDoSample(doSample bool) ImageTextGenerationRequestOption {
	return func(req *ImageTextGenerationRequest) {
		req.Meta["do_sample"] = strconv.FormatBool(doSample)
	}
}

// WithAddGenerationPrompt adds a generation prompt to the input.
func WithAddGenerationPrompt(addPrompt bool) ImageTextGenerationRequestOption {
	return func(req *ImageTextGenerationRequest) {
		req.Meta["add_generation_prompt"] = strconv.FormatBool(addPrompt)
	}
}

// WithStopSequences sets the stop sequences for generation.
func WithStopSequences(stopSequences []string) ImageTextGenerationRequestOption {
	return func(req *ImageTextGenerationRequest) {
		sequences, _ := json.Marshal(stopSequences)
		req.Meta["stop_sequences"] = string(sequences)
	}
}

// WithPrompt sets the text prompt for the generation request.
func WithPrompt(prompt string) ImageTextGenerationRequestOption {
	return func(req *ImageTextGenerationRequest) {
		req.Meta["prompt"] = prompt
	}
}

// WithMessages sets the messages in chat format for the generation request.
func WithMessages(messages []map[string]string) ImageTextGenerationRequestOption {
	return func(req *ImageTextGenerationRequest) {
		msgs, _ := json.Marshal(messages)
		req.Meta["messages"] = string(msgs)
	}
}

// NewImageTextGenerationRequest creates a new image+text generation request.
//
// This function initializes a request with the provided image data and MIME type,
// with optional generation parameters configured via functional options.
//
// Parameters:
//   - payload: The raw image bytes to process
//   - payloadMime: The MIME type of the image (e.g., "image/jpeg", "image/png")
//   - opts: Optional functions to configure generation parameters
//
// Returns:
//   - *ImageTextGenerationRequest: Configured request ready for ForImageTextGeneration()
//   - error: Non-nil if the payload is not a supported image type
//
// Role in project: Factory function for creating VLM requests.
//
// Example:
//
//	imageData, _ := os.ReadFile("cat.jpg")
//	req, err := types.NewImageTextGenerationRequest(imageData, "image/jpeg",
//		types.WithMaxTokens(512),
//		types.WithTemperature(0.0),
//		types.WithMessages([]map[string]string{
//			{"role": "user", "content": "What's in this image?"},
//		}))
//
//	if err != nil {
//		log.Fatalf("Failed to create request: %v", err)
//	}
func NewImageTextGenerationRequest(payload []byte, payloadMime string, opts ...ImageTextGenerationRequestOption) (*ImageTextGenerationRequest, error) {
	if !mimetype.EqualsAny(payloadMime, SupportedImageMimeTypes...) {
		return nil, fmt.Errorf("unsupported payload type: %s", payloadMime)
	}

	req := &ImageTextGenerationRequest{
		Payload:     payload,
		PayloadMime: payloadMime,
		Meta:        make(map[string]string),
	}

	// Set default values
	req.Meta["max_new_tokens"] = "512"
	req.Meta["temperature"] = "0.0"
	req.Meta["top_p"] = "1.0"
	req.Meta["repetition_penalty"] = "1.0"
	req.Meta["do_sample"] = "false"
	req.Meta["add_generation_prompt"] = "true"

	// Apply options
	for _, opt := range opts {
		opt(req)
	}

	return req, nil
}

// ValidFinishReasons defines the acceptable values for the FinishReason field.
var ValidFinishReasons = []string{
	"stop",          // Generation completed normally
	"length",        // Reached max token limit
	"eos_token",     // Encountered end of sequence token
	"stop_sequence", // Encountered predefined stop sequence
	"error",         // Generation terminated due to an error
}

// IsValidFinishReason checks if the provided finish reason is valid.
func (t *TextGenerationV1) IsValidFinishReason() bool {
	for _, reason := range ValidFinishReasons {
		if t.FinishReason == reason {
			return true
		}
	}
	return false
}
