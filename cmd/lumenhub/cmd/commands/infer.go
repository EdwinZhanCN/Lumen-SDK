package commands

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/edwinzhancn/lumen-sdk/cmd/lumenhub/internal"
	"github.com/edwinzhancn/lumen-sdk/pkg/server/rest"
	"github.com/spf13/cobra"
)

// InferCmd is a single, generic CLI entrypoint for making inference requests.
//
// Users build the request with flags:
//
//	--task           (required) : task name, e.g. "semantic_image_embed", "ocr"
//	--service                   : optional service hint written to meta.service, e.g. "clip", "siglip"
//	--payload-file              : path to a binary file to use as payload (recommended for images/audio)
//	--payload-b64               : base64-encoded payload string (alternative to file)
//	--meta                      : JSON object string merged into InferRequest meta
//	--correlation-id            : optional correlation id
//	--output                    : json|yaml|table (default: table)
//
// The command will POST a single `rest.RESTInferRequest` to the daemon's `/v1/infer` endpoint.
var InferCmd = &cobra.Command{
	Use:   "infer",
	Short: "Run a generic inference request against a Lumen Hub daemon",
	Long: `Generic inference command. Build a RESTInferRequest using flags and send it to the daemon.
You may provide payload as a file (--payload-file) or as a base64 string (--payload-b64).
Metadata is provided as a JSON object string (e.g. '{"threshold":"0.5","max_faces":"10"}').`,
	Args: cobra.NoArgs,
	RunE: runInfer,
}

func init() {
	// Request flags
	InferCmd.Flags().String("service", "", "Optional service hint written to meta.service. E.g. bioclip, clip, siglip, ocr, face")
	InferCmd.Flags().String("task", "", "Lumen Hub task name, e.g. semantic_text_embed, semantic_image_embed, bioclip_classify, ocr, face_recognition")
	InferCmd.Flags().String("payload-mime", "application/octet-stream", "Payload MIME type, e.g. text/plain, image/jpeg, application/octet-stream")
	InferCmd.Flags().String("payload-file", "", "Path to payload file (binary). If set, this takes precedence over --payload-b64")
	InferCmd.Flags().String("payload-b64", "", "Base64-encoded payload string (alternative to file)")
	InferCmd.Flags().String("meta", "", "JSON string of meta map (e.g. '{\"threshold\":\"0.5\"}')")
	InferCmd.Flags().String("correlation-id", "", "Optional correlation id for tracing")
	InferCmd.Flags().String("output", "table", "Output format: json|yaml|table")

	// task is now the primary routing key; service is an optional hint.
	_ = InferCmd.MarkFlagRequired("task")
}

// runInfer builds a rest.RESTInferRequest from flags and sends it to the daemon.
func runInfer(cmd *cobra.Command, _ []string) error {
	service, _ := cmd.Flags().GetString("service")
	task, _ := cmd.Flags().GetString("task")
	payloadMIME, _ := cmd.Flags().GetString("payload-mime")
	payloadFile, _ := cmd.Flags().GetString("payload-file")
	payloadB64, _ := cmd.Flags().GetString("payload-b64")
	metaStr, _ := cmd.Flags().GetString("meta")
	corrID, _ := cmd.Flags().GetString("correlation-id")
	outputFormat, _ := cmd.Flags().GetString("output")

	var payload []byte
	var payloadString string
	var err error

	if payloadFile != "" {
		payload, err = os.ReadFile(payloadFile)
		if err != nil {
			return fmt.Errorf("failed to read payload file: %w", err)
		}
		if payloadMIME == "text/plain" {
			payloadString = string(payload)
		} else {
			payloadString = base64.StdEncoding.EncodeToString(payload)
		}
	} else if payloadB64 != "" {
		if payloadMIME == "text/plain" {
			decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(payloadB64))
			if err != nil {
				return fmt.Errorf("failed to decode payload-b64: %w", err)
			}
			payloadString = string(decoded)
		} else {
			payloadString = strings.TrimSpace(payloadB64)
		}
	} else {
		// No payload provided; allow empty payload for services that don't require it.
		payloadString = ""
	}

	// parse meta JSON string into map[string]string
	var meta map[string]string
	if metaStr != "" {
		if err := json.Unmarshal([]byte(metaStr), &meta); err != nil {
			return fmt.Errorf("invalid meta JSON: %w", err)
		}
	} else {
		meta = map[string]string{}
	}
	if service != "" {
		meta["service"] = service
	}

	req := &rest.RESTInferRequest{
		Task:          task,
		PayloadMime:   payloadMIME,
		Payload:       payloadString,
		CorrelationID: corrID,
		Meta:          meta,
	}

	// Create API client (uses global flags for host/port if set on root command)
	client := internal.NewAPIClient(getHostFromCmd(cmd), getPortFromCmd(cmd))

	resp, err := client.PostInfer(req)
	if err != nil {
		return fmt.Errorf("inference request failed: %w", err)
	}

	return outputInferenceResult(resp, outputFormat)
}

// outputInferenceResult renders the APIResponse in the selected format.
// Simple implementations: JSON, YAML-like (manual), or a human table.
func outputInferenceResult(resp *rest.APIResponse, outputFormat string) error {
	switch outputFormat {
	case "json":
		b, err := json.MarshalIndent(resp.Data, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal response data to json: %w", err)
		}
		fmt.Println(string(b))
		return nil
	case "yaml":
		// Minimal YAML-ish printing
		fmt.Printf("success: %t\n", resp.Success)
		if resp.Timestamp != "" {
			fmt.Printf("timestamp: %s\n", resp.Timestamp)
		}
		if resp.RequestID != "" {
			fmt.Printf("request_id: %s\n", resp.RequestID)
		}
		if resp.Data != nil {
			fmt.Printf("data:\n")
			if m, ok := resp.Data.(map[string]interface{}); ok {
				for k, v := range m {
					fmt.Printf("  %s: %v\n", k, v)
				}
			} else {
				// fallback to JSON blob
				b, _ := json.MarshalIndent(resp.Data, "  ", "  ")
				fmt.Printf("  raw: %s\n", string(b))
			}
		}
		return nil
	default:
		// Table/human friendly
		fmt.Printf("Success: %t\n", resp.Success)
		if resp.Timestamp != "" {
			fmt.Printf("Timestamp: %s\n", resp.Timestamp)
		}
		if resp.RequestID != "" {
			fmt.Printf("Request ID: %s\n", resp.RequestID)
		}
		if resp.Data != nil {
			fmt.Printf("\nData:\n")
			if m, ok := resp.Data.(map[string]interface{}); ok {
				for k, v := range m {
					fmt.Printf("  %s: %v\n", k, v)
				}
			} else {
				b, _ := json.MarshalIndent(resp.Data, "", "  ")
				fmt.Println(string(b))
			}
		}
		return nil
	}
}
