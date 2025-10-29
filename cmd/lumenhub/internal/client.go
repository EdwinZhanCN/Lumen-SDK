package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/edwinzhancn/lumen-sdk/pkg/server/rest"
)

// APIClient is the HTTP client for communicating with lumenhubd daemon
type APIClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewAPIClient creates a new API client
func NewAPIClient(host string, port int) *APIClient {
	baseURL := fmt.Sprintf("http://%s:%d", host, port)

	return &APIClient{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Get performs a GET request to the API
func (c *APIClient) Get(endpoint string) (*rest.APIResponse, error) {
	url := c.BaseURL + endpoint

	resp, err := c.HTTPClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	return c.parseResponse(resp)
}

// Post performs a POST request to the API
func (c *APIClient) Post(endpoint string, body interface{}) (*rest.APIResponse, error) {
	url := c.BaseURL + endpoint

	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	resp, err := c.HTTPClient.Post(url, "application/json", bodyReader)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	return c.parseResponse(resp)
}

// parseResponse parses the HTTP response into APIResponse
func (c *APIClient) parseResponse(resp *http.Response) (*rest.APIResponse, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var apiResp rest.APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Check for API-level errors
	if !apiResp.Success && apiResp.Error != nil {
		return nil, fmt.Errorf("API error [%s]: %s", apiResp.Error.Code, apiResp.Error.Message)
	}

	// Check HTTP status
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP error %d: %s", resp.StatusCode, resp.Status)
	}

	return &apiResp, nil
}

// GetNodes retrieves the list of nodes
func (c *APIClient) GetNodes() (*rest.APIResponse, error) {
	return c.Get("/api/v1/nodes")
}

// GetHealth retrieves the health status
func (c *APIClient) GetHealth() (*rest.APIResponse, error) {
	return c.Get("/api/v1/health")
}

// GetMetrics retrieves the metrics
func (c *APIClient) GetMetrics() (*rest.APIResponse, error) {
	return c.Get("/api/v1/metrics")
}

// PostEmbedding performs text embedding
func (c *APIClient) PostEmbedding(request *rest.EmbedRequest) (*rest.APIResponse, error) {
	return c.Post("/api/v1/embed", request)
}

// PostDetection performs object detection
func (c *APIClient) PostDetection(request *rest.DetectRequest) (*rest.APIResponse, error) {
	return c.Post("/api/v1/detect", request)
}

// PostOCR performs OCR
func (c *APIClient) PostOCR(request *rest.OCRRequest) (*rest.APIResponse, error) {
	return c.Post("/api/v1/ocr", request)
}

// PostTTS performs text-to-speech
func (c *APIClient) PostTTS(request *rest.TTSRequest) (*rest.APIResponse, error) {
	return c.Post("/api/v1/tts", request)
}
