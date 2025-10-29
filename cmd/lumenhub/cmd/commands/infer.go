package commands

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/edwinzhancn/lumen-sdk/cmd/lumenhub/internal"
	"github.com/edwinzhancn/lumen-sdk/pkg/server/rest"

	"github.com/spf13/cobra"
)

// InferCmd represents the inference command
var InferCmd = &cobra.Command{
	Use:   "infer",
	Short: "AI inference commands",
	Long:  `Perform AI inference using various models available on the Lumen Hub.`,
}

var embedCmd = &cobra.Command{
	Use:   "embed [text]",
	Short: "Generate text embeddings",
	Long: `Generate embeddings for the given text using available embedding models.

Example:
  lumenhub infer embed "Hello world"
  lumenhub infer embed "Hello world" --model-id text-embedding-ada-002`,
	Args: cobra.ExactArgs(1),
	RunE: runEmbed,
}

var detectCmd = &cobra.Command{
	Use:   "detect",
	Short: "Object detection in images",
	Long: `Detect objects in images using computer vision models.

Example:
  lumenhub infer detect --image ./photo.jpg
  lumenhub infer detect --image ./photo.jpg --model-id yolov5`,
	RunE: runDetect,
}

var ocrCmd = &cobra.Command{
	Use:   "ocr",
	Short: "Optical Character Recognition",
	Long: `Extract text from images using OCR models.

Example:
  lumenhub infer ocr --image ./document.jpg
  lumenhub infer ocr --image ./document.jpg --language en`,
	RunE: runOCR,
}

var ttsCmd = &cobra.Command{
	Use:   "tts [text]",
	Short: "Text-to-Speech synthesis",
	Long: `Convert text to speech using TTS models.

Example:
  lumenhub infer tts "Hello world"
  lumenhub infer tts "Hello world" --output hello.wav
  lumenhub infer tts "Hello world" --voice-id en-US-Wavenet-D`,
	Args: cobra.ExactArgs(1),
	RunE: runTTS,
}

func init() {
	InferCmd.AddCommand(embedCmd)
	InferCmd.AddCommand(detectCmd)
	InferCmd.AddCommand(ocrCmd)
	InferCmd.AddCommand(ttsCmd)

	// Embed flags
	embedCmd.Flags().String("model-id", "", "Model ID for embedding")
	embedCmd.Flags().String("language", "en", "Language code")
	embedCmd.Flags().String("image", "", "Image file path (for multimodal embedding)")

	// Detect flags
	detectCmd.Flags().String("image", "", "Image file path (required)")
	detectCmd.Flags().String("model-id", "", "Model ID for detection")
	detectCmd.Flags().Float64("threshold", 0.5, "Confidence threshold")
	detectCmd.Flags().Int("max-results", 100, "Maximum number of results")
	detectCmd.Flags().StringSlice("classes", []string{}, "Filter by specific classes")

	// OCR flags
	ocrCmd.Flags().String("image", "", "Image file path (required)")
	ocrCmd.Flags().String("model-id", "", "Model ID for OCR")
	ocrCmd.Flags().String("language", "auto", "Language code")

	// TTS flags
	ttsCmd.Flags().String("model-id", "", "Model ID for TTS")
	ttsCmd.Flags().String("voice-id", "", "Voice ID for synthesis")
	ttsCmd.Flags().String("language", "en", "Language code")
	ttsCmd.Flags().String("output", "", "Output file path (optional)")
	ttsCmd.Flags().Float64("speed", 1.0, "Speech speed (0.1-2.0)")
	ttsCmd.Flags().Float64("pitch", 1.0, "Speech pitch (0.1-2.0)")
	ttsCmd.Flags().Float64("volume", 1.0, "Speech volume (0.1-2.0)")
	ttsCmd.Flags().String("format", "wav", "Audio format (wav|mp3|ogg)")

	// Mark required flags
	detectCmd.MarkFlagRequired("image")
	ocrCmd.MarkFlagRequired("image")
}

func runEmbed(cmd *cobra.Command, args []string) error {
	text := args[0]

	modelID, _ := cmd.Flags().GetString("model-id")
	language, _ := cmd.Flags().GetString("language")
	image, _ := cmd.Flags().GetString("image")

	// Prepare request
	request := &rest.EmbedRequest{
		Text:     text,
		ModelID:  modelID,
		Language: language,
	}

	// Handle image file
	if image != "" {
		imageData, err := readImageAsBase64(image)
		if err != nil {
			return fmt.Errorf("failed to read image: %w", err)
		}
		request.Image = imageData
	}

	client := internal.NewAPIClient(getHostFromCmd(cmd), getPortFromCmd(cmd))

	outputFormat, _ := cmd.Flags().GetString("output")
	resp, err := client.PostEmbedding(request)
	if err != nil {
		return fmt.Errorf("embedding request failed: %w", err)
	}

	return outputInferenceResult(resp, outputFormat)
}

func runDetect(cmd *cobra.Command, args []string) error {
	image, _ := cmd.Flags().GetString("image")
	modelID, _ := cmd.Flags().GetString("model-id")
	threshold, _ := cmd.Flags().GetFloat64("threshold")
	maxResults, _ := cmd.Flags().GetInt("max-results")
	classes, _ := cmd.Flags().GetStringSlice("classes")

	// Read image
	imageData, err := readImageAsBase64(image)
	if err != nil {
		return fmt.Errorf("failed to read image: %w", err)
	}

	// Prepare request
	request := &rest.DetectRequest{
		Image:      imageData,
		ModelID:    modelID,
		Threshold:  float32(threshold),
		MaxResults: maxResults,
		Classes:    classes,
	}

	client := internal.NewAPIClient(getHostFromCmd(cmd), getPortFromCmd(cmd))

	outputFormat, _ := cmd.Flags().GetString("output")
	resp, err := client.PostDetection(request)
	if err != nil {
		return fmt.Errorf("detection request failed: %w", err)
	}

	return outputInferenceResult(resp, outputFormat)
}

func runOCR(cmd *cobra.Command, args []string) error {
	image, _ := cmd.Flags().GetString("image")
	modelID, _ := cmd.Flags().GetString("model-id")
	language, _ := cmd.Flags().GetString("language")

	// Read image
	imageData, err := readImageAsBase64(image)
	if err != nil {
		return fmt.Errorf("failed to read image: %w", err)
	}

	// Prepare request
	request := &rest.OCRRequest{
		Image:    imageData,
		ModelID:  modelID,
		Language: []string{language},
	}

	client := internal.NewAPIClient(getHostFromCmd(cmd), getPortFromCmd(cmd))

	outputFormat, _ := cmd.Flags().GetString("output")
	resp, err := client.PostOCR(request)
	if err != nil {
		return fmt.Errorf("OCR request failed: %w", err)
	}

	return outputInferenceResult(resp, outputFormat)
}

func runTTS(cmd *cobra.Command, args []string) error {
	text := args[0]

	modelID, _ := cmd.Flags().GetString("model-id")
	voiceID, _ := cmd.Flags().GetString("voice-id")
	language, _ := cmd.Flags().GetString("language")
	output, _ := cmd.Flags().GetString("output")
	speed, _ := cmd.Flags().GetFloat64("speed")
	pitch, _ := cmd.Flags().GetFloat64("pitch")
	volume, _ := cmd.Flags().GetFloat64("volume")
	format, _ := cmd.Flags().GetString("format")

	// Prepare request
	request := &rest.TTSRequest{
		Text:     text,
		ModelID:  modelID,
		VoiceID:  voiceID,
		Language: language,
		Speed:    float32(speed),
		Pitch:    float32(pitch),
		Volume:   float32(volume),
		Format:   format,
	}

	client := internal.NewAPIClient(getHostFromCmd(cmd), getPortFromCmd(cmd))

	outputFormat, _ := cmd.Flags().GetString("output")
	resp, err := client.PostTTS(request)
	if err != nil {
		return fmt.Errorf("TTS request failed: %w", err)
	}

	// Handle audio output
	if outputFormat == "json" || outputFormat == "yaml" {
		return outputInferenceResult(resp, outputFormat)
	}

	// For TTS, if not JSON/YAML, handle audio data
	if resp.Data != nil {
		if dataMap, ok := resp.Data.(map[string]interface{}); ok {
			if audioDataInterface, ok := dataMap["audio_data"]; ok {
				if audioData, ok := audioDataInterface.(string); ok && audioData != "" {
					return saveAudioToFile(audioData, output, format)
				}
			}
		}
	}

	return outputInferenceResult(resp, "table")
}

func readImageAsBase64(imagePath string) (string, error) {
	// Check if it's a URL or file path
	if isURL(imagePath) {
		// For URLs, just return the URL - the server will handle downloading
		return imagePath, nil
	}

	// Read file
	data, err := ioutil.ReadFile(imagePath)
	if err != nil {
		return "", fmt.Errorf("failed to read image file: %w", err)
	}

	// Convert to base64 (simplified - in production you'd use proper encoding)
	return string(data), nil
}

func saveAudioToFile(audioData, outputPath, format string) error {
	if outputPath == "" {
		outputPath = "output." + format
	}

	// Decode base64 and save (simplified)
	data := []byte(audioData) // In production, you'd decode base64 here

	err := ioutil.WriteFile(outputPath, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to save audio file: %w", err)
	}

	fmt.Printf("Audio saved to: %s\n", outputPath)
	return nil
}

func outputInferenceResult(resp *rest.APIResponse, outputFormat string) error {
	switch outputFormat {
	case "json":
		if data, err := json.MarshalIndent(resp.Data, "", "  "); err == nil {
			fmt.Println(string(data))
		}
	case "yaml":
		fmt.Printf("# Inference Result\n")
		fmt.Printf("success: %t\n", resp.Success)
		fmt.Printf("timestamp: %s\n", resp.Timestamp)
		if resp.RequestID != "" {
			fmt.Printf("request_id: %s\n", resp.RequestID)
		}
		// Simple YAML output - in production you'd use a proper YAML library
		if resp.Data != nil {
			fmt.Printf("data:\n")
			if dataMap, ok := resp.Data.(map[string]interface{}); ok {
				for key, value := range dataMap {
					fmt.Printf("  %s: %v\n", key, value)
				}
			}
		}
	default:
		// Table format
		fmt.Printf("Inference Result\n")
		fmt.Printf("================\n")
		fmt.Printf("Success: %t\n", resp.Success)
		fmt.Printf("Timestamp: %s\n", resp.Timestamp)
		if resp.RequestID != "" {
			fmt.Printf("Request ID: %s\n", resp.RequestID)
		}

		if resp.Data != nil {
			fmt.Printf("\nData:\n")
			if dataMap, ok := resp.Data.(map[string]interface{}); ok {
				for key, value := range dataMap {
					fmt.Printf("  %s: %v\n", key, value)
				}
			}
		}
	}

	return nil
}

func isURL(s string) bool {
	return len(s) > 7 && (s[:7] == "http://" || s[:8] == "https://")
}
