package rest

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"Lumen-SDK/pkg/client"
	"Lumen-SDK/pkg/codec"
	pb "Lumen-SDK/proto"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

// Handler REST处理器
type Handler struct {
	client        *client.LumenClient
	codecRegistry *codec.CodecRegistry
	logger        *zap.Logger
}

// NewHandler 创建新的处理器
func NewHandler(lumenClient *client.LumenClient, codecRegistry *codec.CodecRegistry, logger *zap.Logger) *Handler {
	return &Handler{
		client:        lumenClient,
		codecRegistry: codecRegistry,
		logger:        logger,
	}
}

// APIResponse 通用API响应结构
type APIResponse struct {
	Success   bool        `json:"success"`
	Data      interface{} `json:"data,omitempty"`
	Error     *APIError   `json:"error,omitempty"`
	RequestID string      `json:"request_id"`
	Timestamp time.Time   `json:"timestamp"`
}

// APIError API错误结构
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// EmbedRequest 嵌入请求
type EmbedRequest struct {
	Text     string `json:"text,omitempty"`
	Image    string `json:"image,omitempty"` // Base64或URL
	ModelID  string `json:"model_id,omitempty"`
	Language string `json:"language,omitempty"`
}

// EmbedResponse 嵌入响应
type EmbedResponse struct {
	Vector    []float32 `json:"vector"`
	Dimension int       `json:"dimension"`
	ModelID   string    `json:"model_id"`
}

// DetectRequest 检测请求
type DetectRequest struct {
	Image      string                 `json:"image"` // Base64或URL
	ModelID    string                 `json:"model_id,omitempty"`
	Threshold  float32                `json:"threshold,omitempty"`
	MaxResults int                    `json:"max_results,omitempty"`
	Classes    []string               `json:"classes,omitempty"`
	Options    map[string]interface{} `json:"options,omitempty"`
}

// DetectResponse 检测响应
type DetectResponse struct {
	Detections []*DetectionResult `json:"detections"`
	Count      int                `json:"count"`
	ModelID    string             `json:"model_id"`
}

// DetectionResult 检测结果
type DetectionResult struct {
	Box        *BoundingBox `json:"box"`
	ClassID    int          `json:"class_id"`
	ClassName  string       `json:"class_name"`
	Confidence float32      `json:"confidence"`
}

// BoundingBox 边界框
type BoundingBox struct {
	XMin float32 `json:"xmin"`
	YMin float32 `json:"ymin"`
	XMax float32 `json:"xmax"`
	YMax float32 `json:"ymax"`
}

// OCRRequest OCR请求
type OCRRequest struct {
	Image    string                 `json:"image"` // Base64或URL
	Language []string               `json:"language,omitempty"`
	ModelID  string                 `json:"model_id,omitempty"`
	Options  map[string]interface{} `json:"options,omitempty"`
}

// OCRResponse OCR响应
type OCRResponse struct {
	TextBlocks []*TextBlock `json:"text_blocks"`
	FullText   string       `json:"full_text"`
	Confidence float32      `json:"confidence"`
	ModelID    string       `json:"model_id"`
}

// TextBlock 文本块
type TextBlock struct {
	Box        *BoundingBox `json:"box"`
	Text       string       `json:"text"`
	Confidence float32      `json:"confidence"`
	Language   string       `json:"language,omitempty"`
}

// TTSRequest TTS请求
type TTSRequest struct {
	Text     string                 `json:"text"`
	VoiceID  string                 `json:"voice_id,omitempty"`
	ModelID  string                 `json:"model_id,omitempty"`
	Language string                 `json:"language,omitempty"`
	Speed    float32                `json:"speed,omitempty"`
	Pitch    float32                `json:"pitch,omitempty"`
	Volume   float32                `json:"volume,omitempty"`
	Format   string                 `json:"format,omitempty"`
	Options  map[string]interface{} `json:"options,omitempty"`
}

// TTSResponse TTS响应
type TTSResponse struct {
	AudioData  string  `json:"audio_data"`
	Format     string  `json:"format"`
	SampleRate int     `json:"sample_rate"`
	Duration   float64 `json:"duration"`
	VoiceID    string  `json:"voice_id"`
	ModelID    string  `json:"model_id"`
}

// HandleEmbed 处理嵌入请求
func (h *Handler) HandleEmbed(c *fiber.Ctx) error {
	requestID := generateRequestID()
	startTime := time.Now()

	h.logger.Info("handling embed request")

	// 解析请求
	var req EmbedRequest
	if err := c.BodyParser(&req); err != nil {
		return h.sendError(c, requestID, "invalid_request", "Failed to parse request body", err.Error())
	}

	// 验证请求
	if err := h.validateEmbedRequest(&req); err != nil {
		return h.sendError(c, requestID, "validation_error", "Request validation failed", err.Error())
	}

	// 准备Lumen客户端请求
	lumenReq, err := h.buildEmbedRequest(&req)
	if err != nil {
		return h.sendError(c, requestID, "build_request_error", "Failed to build request", err.Error())
	}

	// 执行推理
	lumenResp, err := h.client.Infer(c.Context(), lumenReq)
	if err != nil {
		return h.sendError(c, requestID, "inference_error", "Inference failed", err.Error())
	}

	// 解析响应
	resp, err := h.parseEmbedResponse(lumenResp)
	if err != nil {
		return h.sendError(c, requestID, "parse_response_error", "Failed to parse response", err.Error())
	}

	h.logger.Info("embed request completed",
		zap.String("request_id", requestID),
		zap.Int64("duration_ms", time.Since(startTime).Milliseconds()))

	return h.sendSuccess(c, requestID, resp)
}

// HandleDetect 处理检测请求
func (h *Handler) HandleDetect(c *fiber.Ctx) error {
	requestID := generateRequestID()
	startTime := time.Now()

	h.logger.Info("handling detect request", zap.String("request_id", requestID))

	// 解析请求
	var req DetectRequest
	if err := c.BodyParser(&req); err != nil {
		return h.sendError(c, requestID, "invalid_request", "Failed to parse request body", err.Error())
	}

	// 验证请求
	if err := h.validateDetectRequest(&req); err != nil {
		return h.sendError(c, requestID, "validation_error", "Request validation failed", err.Error())
	}

	// 准备Lumen客户端请求
	lumenReq, err := h.buildDetectRequest(&req)
	if err != nil {
		return h.sendError(c, requestID, "build_request_error", "Failed to build request", err.Error())
	}

	// 执行推理
	lumenResp, err := h.client.Infer(c.Context(), lumenReq)
	if err != nil {
		return h.sendError(c, requestID, "inference_error", "Inference failed", err.Error())
	}

	// 解析响应
	resp, err := h.parseDetectResponse(lumenResp)
	if err != nil {
		return h.sendError(c, requestID, "parse_response_error", "Failed to parse response", err.Error())
	}

	h.logger.Info("detect request completed",
		zap.String("request_id", requestID),
		zap.Int64("duration_ms", time.Since(startTime).Milliseconds()),
		zap.Int("detections_count", len(resp.Detections)))

	return h.sendSuccess(c, requestID, resp)
}

// HandleOCR 处理OCR请求
func (h *Handler) HandleOCR(c *fiber.Ctx) error {
	requestID := generateRequestID()
	startTime := time.Now()

	h.logger.Info("handling ocr request", zap.String("request_id", requestID))

	// 解析请求
	var req OCRRequest
	if err := c.BodyParser(&req); err != nil {
		return h.sendError(c, requestID, "invalid_request", "Failed to parse request body", err.Error())
	}

	// 验证请求
	if err := h.validateOCRRequest(&req); err != nil {
		return h.sendError(c, requestID, "validation_error", "Request validation failed", err.Error())
	}

	// 准备Lumen客户端请求
	lumenReq, err := h.buildOCRRequest(&req)
	if err != nil {
		return h.sendError(c, requestID, "build_request_error", "Failed to build request", err.Error())
	}

	// 执行推理
	lumenResp, err := h.client.Infer(c.Context(), lumenReq)
	if err != nil {
		return h.sendError(c, requestID, "inference_error", "Inference failed", err.Error())
	}

	// 解析响应
	resp, err := h.parseOCRResponse(lumenResp)
	if err != nil {
		return h.sendError(c, requestID, "parse_response_error", "Failed to parse response", err.Error())
	}

	// 记录成功
	h.logger.Info("ocr request completed",
		zap.String("request_id", requestID),
		zap.Int64("duration_ms", time.Since(startTime).Milliseconds()),
		zap.Int("text_blocks_count", len(resp.TextBlocks)))

	return h.sendSuccess(c, requestID, resp)
}

// HandleTTS 处理TTS请求
func (h *Handler) HandleTTS(c *fiber.Ctx) error {
	requestID := generateRequestID()
	startTime := time.Now()

	h.logger.Info("handling tts request", zap.String("request_id", requestID))

	// 解析请求
	var req TTSRequest
	if err := c.BodyParser(&req); err != nil {
		return h.sendError(c, requestID, "invalid_request", "Failed to parse request body", err.Error())
	}

	// 验证请求
	if err := h.validateTTSRequest(&req); err != nil {
		return h.sendError(c, requestID, "validation_error", "Request validation failed", err.Error())
	}

	// 准备Lumen客户端请求
	lumenReq, err := h.buildTTSRequest(&req)
	if err != nil {
		return h.sendError(c, requestID, "build_request_error", "Failed to build request", err.Error())
	}

	// 执行推理
	lumenResp, err := h.client.Infer(c.Context(), lumenReq)
	if err != nil {
		return h.sendError(c, requestID, "inference_error", "Inference failed", err.Error())
	}

	// 解析响应
	resp, err := h.parseTTSResponse(lumenResp)
	if err != nil {
		return h.sendError(c, requestID, "parse_response_error", "Failed to parse response", err.Error())
	}

	// 记录成功
	h.logger.Info("tts request completed",
		zap.String("request_id", requestID),
		zap.Int64("duration_ms", time.Since(startTime).Milliseconds()),
		zap.Float64("audio_duration", resp.Duration))

	return h.sendSuccess(c, requestID, resp)
}

// HandleNodes 获取节点列表
func (h *Handler) HandleNodes(c *fiber.Ctx) error {
	requestID := generateRequestID()

	nodes := h.client.GetNodes()

	// 转换为API格式
	apiNodes := make([]*client.NodeInfo, len(nodes))
	for i, node := range nodes {
		apiNodes[i] = &client.NodeInfo{
			ID:       node.ID,
			Name:     node.Name,
			Address:  node.Address,
			Status:   node.Status,
			LastSeen: node.LastSeen,
			Tasks:    node.Tasks,
			Runtime:  node.Runtime,
		}
	}

	return h.sendSuccess(c, requestID, map[string]interface{}{
		"nodes": apiNodes,
		"count": len(apiNodes),
	})
}

// HandleHealth 健康检查
func (h *Handler) HandleHealth(c *fiber.Ctx) error {
	requestID := generateRequestID()

	// 获取客户端指标
	metrics := h.client.GetMetrics()

	// 构建健康状态
	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now(),
		"version":   "1.0.0",
		"uptime":    time.Since(startTime),
		"metrics": map[string]interface{}{
			"total_requests":      metrics.TotalRequests,
			"successful_requests": metrics.SuccessfulRequests,
			"failed_requests":     metrics.FailedRequests,
			"active_nodes":        metrics.ActiveNodes,
			"total_nodes":         metrics.TotalNodes,
			"error_rate":          metrics.ErrorRate,
			"throughput_qps":      metrics.ThroughputQPS,
		},
	}

	// 检查是否有错误
	if metrics.ErrorRate > 0.1 { // 错误率超过10%
		health["status"] = "degraded"
	}

	if metrics.ActiveNodes == 0 {
		health["status"] = "unhealthy"
	}

	statusCode := fiber.StatusOK
	if health["status"] == "degraded" {
		statusCode = 200 // 仍然返回200，但状态为degraded
	} else if health["status"] == "unhealthy" {
		statusCode = fiber.StatusServiceUnavailable
	}

	c.Status(statusCode)
	return h.sendSuccess(c, requestID, health)
}

// HandleMetrics 获取指标
func (h *Handler) HandleMetrics(c *fiber.Ctx) error {
	requestID := generateRequestID()

	// 获取客户端指标
	metrics := h.client.GetMetrics()

	// 获取节点详情
	nodes := h.client.GetNodes()
	nodeMetrics := make(map[string]interface{})
	for _, node := range nodes {
		nodeMetrics[node.ID] = map[string]interface{}{
			"name":      node.Name,
			"address":   node.Address,
			"status":    node.Status,
			"last_seen": node.LastSeen,
			"tasks":     node.Tasks,
			"runtime":   node.Runtime,
		}
	}

	result := map[string]interface{}{
		"client_metrics": metrics,
		"node_metrics":   nodeMetrics,
		"timestamp":      time.Now(),
	}

	return h.sendSuccess(c, requestID, result)
}

// 私有方法

// sendSuccess 发送成功响应
func (h *Handler) sendSuccess(c *fiber.Ctx, requestID string, data interface{}) error {
	response := APIResponse{
		Success:   true,
		Data:      data,
		RequestID: requestID,
		Timestamp: time.Now(),
	}

	return c.Status(fiber.StatusOK).JSON(response)
}

// sendError 发送错误响应
func (h *Handler) sendError(c *fiber.Ctx, requestID, code, message, details string) error {
	h.logger.Error("request failed",
		zap.String("request_id", requestID),
		zap.String("error_code", code),
		zap.String("error_message", message),
		zap.String("error_details", details))

	response := APIResponse{
		Success: false,
		Error: &APIError{
			Code:    code,
			Message: message,
			Details: details,
		},
		RequestID: requestID,
		Timestamp: time.Now(),
	}

	return c.Status(fiber.StatusBadRequest).JSON(response)
}

// validateEmbedRequest 验证嵌入请求
func (h *Handler) validateEmbedRequest(req *EmbedRequest) error {
	if req.Text == "" && req.Image == "" {
		return fmt.Errorf("either text or image must be provided")
	}

	if req.Text != "" && len(req.Text) > 8000 {
		return fmt.Errorf("text too long, maximum 8000 characters")
	}

	if req.Image != "" {
		// 验证图像格式（Base64或URL）
		if !isValidImageInput(req.Image) {
			return fmt.Errorf("invalid image format, must be valid base64 or URL")
		}
	}

	return nil
}

// validateDetectRequest 验证检测请求
func (h *Handler) validateDetectRequest(req *DetectRequest) error {
	if req.Image == "" {
		return fmt.Errorf("image is required")
	}

	if !isValidImageInput(req.Image) {
		return fmt.Errorf("invalid image format, must be valid base64 or URL")
	}

	if req.Threshold < 0 || req.Threshold > 1 {
		return fmt.Errorf("threshold must be between 0 and 1")
	}

	if req.MaxResults < 0 || req.MaxResults > 1000 {
		return fmt.Errorf("max_results must be between 0 and 1000")
	}

	return nil
}

// validateOCRRequest 验证OCR请求
func (h *Handler) validateOCRRequest(req *OCRRequest) error {
	if req.Image == "" {
		return fmt.Errorf("image is required")
	}

	if !isValidImageInput(req.Image) {
		return fmt.Errorf("invalid image format, must be valid base64 or URL")
	}

	if len(req.Language) > 10 {
		return fmt.Errorf("too many languages specified")
	}

	return nil
}

// validateTTSRequest 验证TTS请求
func (h *Handler) validateTTSRequest(req *TTSRequest) error {
	if req.Text == "" {
		return fmt.Errorf("text is required")
	}

	if len(req.Text) > 10000 {
		return fmt.Errorf("text too long, maximum 10000 characters")
	}

	if req.Speed < 0.1 || req.Speed > 3.0 {
		return fmt.Errorf("speed must be between 0.1 and 3.0")
	}

	if req.Pitch < -20.0 || req.Pitch > 20.0 {
		return fmt.Errorf("pitch must be between -20.0 and 20.0")
	}

	if req.Volume < 0.0 || req.Volume > 1.0 {
		return fmt.Errorf("volume must be between 0.0 and 1.0")
	}

	return nil
}

// buildEmbedRequest 构建嵌入请求
func (h *Handler) buildEmbedRequest(req *EmbedRequest) (*pb.InferRequest, error) {
	var payload []byte
	var mime string

	if req.Text != "" {
		payload = []byte(req.Text)
		mime = "text/plain"
	} else if req.Image != "" {
		imageData, err := h.processImageInput(req.Image)
		if err != nil {
			return nil, fmt.Errorf("failed to process image: %w", err)
		}
		payload = imageData
		mime = "image/jpeg"
	} else {
		return nil, fmt.Errorf("either text or image must be provided")
	}

	meta := make(map[string]string)
	if req.ModelID != "" {
		meta["model_id"] = req.ModelID
	}
	if req.Language != "" {
		meta["language"] = req.Language
	}

	return &pb.InferRequest{
		CorrelationId: generateRequestID(),
		Task:          "embed",
		Payload:       payload,
		PayloadMime:   mime,
		Meta:          meta,
	}, nil
}

// buildDetectRequest 构建检测请求
func (h *Handler) buildDetectRequest(req *DetectRequest) (*pb.InferRequest, error) {
	imageData, err := h.processImageInput(req.Image)
	if err != nil {
		return nil, fmt.Errorf("failed to process image: %w", err)
	}

	meta := make(map[string]string)
	if req.ModelID != "" {
		meta["model_id"] = req.ModelID
	}
	if req.Threshold > 0 {
		meta["threshold"] = fmt.Sprintf("%f", req.Threshold)
	}
	if req.MaxResults > 0 {
		meta["max_results"] = fmt.Sprintf("%d", req.MaxResults)
	}
	if len(req.Classes) > 0 {
		classesJSON, _ := json.Marshal(req.Classes)
		meta["classes"] = string(classesJSON)
	}

	return &pb.InferRequest{
		CorrelationId: generateRequestID(),
		Task:          "detect",
		Payload:       imageData,
		PayloadMime:   "image/jpeg",
		Meta:          meta,
	}, nil
}

// buildOCRRequest 构建OCR请求
func (h *Handler) buildOCRRequest(req *OCRRequest) (*pb.InferRequest, error) {
	imageData, err := h.processImageInput(req.Image)
	if err != nil {
		return nil, fmt.Errorf("failed to process image: %w", err)
	}

	meta := make(map[string]string)
	if req.ModelID != "" {
		meta["model_id"] = req.ModelID
	}
	if len(req.Language) > 0 {
		languagesJSON, _ := json.Marshal(req.Language)
		meta["languages"] = string(languagesJSON)
	}

	return &pb.InferRequest{
		CorrelationId: generateRequestID(),
		Task:          "ocr",
		Payload:       imageData,
		PayloadMime:   "image/jpeg",
		Meta:          meta,
	}, nil
}

// buildTTSRequest 构建TTS请求
func (h *Handler) buildTTSRequest(req *TTSRequest) (*pb.InferRequest, error) {
	payload := []byte(req.Text)

	meta := make(map[string]string)
	if req.ModelID != "" {
		meta["model_id"] = req.ModelID
	}
	if req.VoiceID != "" {
		meta["voice_id"] = req.VoiceID
	}
	if req.Language != "" {
		meta["language"] = req.Language
	}
	if req.Speed > 0 {
		meta["speed"] = fmt.Sprintf("%f", req.Speed)
	}
	if req.Pitch != 0 {
		meta["pitch"] = fmt.Sprintf("%f", req.Pitch)
	}
	if req.Volume > 0 {
		meta["volume"] = fmt.Sprintf("%f", req.Volume)
	}
	if req.Format != "" {
		meta["format"] = req.Format
	}

	return &pb.InferRequest{
		CorrelationId: generateRequestID(),
		Task:          "tts",
		Payload:       payload,
		PayloadMime:   "text/plain",
		Meta:          meta,
	}, nil
}

// parseEmbedResponse 解析嵌入响应
func (h *Handler) parseEmbedResponse(resp *pb.InferResponse) (*EmbedResponse, error) {
	if resp.Error != nil {
		return nil, fmt.Errorf("inference failed: %s", resp.Error.Message)
	}

	// 解析向量数据
	var vector []float32
	if err := json.Unmarshal(resp.Result, &vector); err != nil {
		return nil, fmt.Errorf("failed to parse vector: %w", err)
	}

	return &EmbedResponse{
		Vector:    vector,
		Dimension: len(vector),
		ModelID:   resp.Meta["model_id"],
	}, nil
}

// parseOCRResponse 解析OCR响应
func (h *Handler) parseOCRResponse(resp *pb.InferResponse) (*OCRResponse, error) {
	if resp.Error != nil {
		return nil, fmt.Errorf("inference failed: %s", resp.Error.Message)
	}

	// 解析OCR结果
	var ocrResp struct {
		TextBlocks []*TextBlock `json:"text_blocks"`
		FullText   string       `json:"full_text"`
		Confidence float32      `json:"confidence"`
	}

	if err := json.Unmarshal(resp.Result, &ocrResp); err != nil {
		return nil, fmt.Errorf("failed to parse OCR results: %w", err)
	}

	return &OCRResponse{
		TextBlocks: ocrResp.TextBlocks,
		FullText:   ocrResp.FullText,
		Confidence: ocrResp.Confidence,
		ModelID:    resp.Meta["model_id"],
	}, nil
}

func (h *Handler) parseDetectResponse(resp *pb.InferResponse) (*DetectResponse, error) {
	if resp.Error != nil {
		return nil, fmt.Errorf("inference failed: %s", resp.Error.Message)
	}

	// 解析检测结果
	var detectResp struct {
		Detections []*DetectionResult `json:"detections"`
		Count      int                `json:"count"`
	}

	if err := json.Unmarshal(resp.Result, &detectResp); err != nil {
		return nil, fmt.Errorf("failed to parse detection results: %w", err)
	}

	return &DetectResponse{
		Detections: detectResp.Detections,
		Count:      detectResp.Count,
		ModelID:    resp.Meta["model_id"],
	}, nil
}

// parseTTSResponse 解析TTS响应
func (h *Handler) parseTTSResponse(resp *pb.InferResponse) (*TTSResponse, error) {
	if resp.Error != nil {
		return nil, fmt.Errorf("inference failed: %s", resp.Error.Message)
	}

	// 解析TTS结果
	var ttsResp struct {
		AudioData  []byte  `json:"audio_data"`
		Format     string  `json:"format"`
		SampleRate int     `json:"sample_rate"`
		Duration   float64 `json:"duration"`
	}

	if err := json.Unmarshal(resp.Result, &ttsResp); err != nil {
		return nil, fmt.Errorf("failed to parse TTS results: %w", err)
	}

	// 使用编码器将音频数据编码为base64字符串
	audioBase64, err := codec.EncodeData("text/plain", ttsResp.AudioData)
	if err != nil {
		return nil, fmt.Errorf("failed to encode audio data to base64: %w", err)
	}

	return &TTSResponse{
		AudioData:  string(audioBase64),
		Format:     ttsResp.Format,
		SampleRate: ttsResp.SampleRate,
		Duration:   ttsResp.Duration,
		ModelID:    resp.Meta["model_id"],
		VoiceID:    resp.Meta["voice_id"],
	}, nil
}

// processImageInput 处理图像输入
func (h *Handler) processImageInput(imageInput string) ([]byte, error) {
	if strings.HasPrefix(imageInput, "data:image/") {
		// Base64 Data URL格式
		parts := strings.SplitN(imageInput, ",", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid data URL format")
		}
		// 使用编解码器进行base64解码
		var decoded []byte
		err := h.codecRegistry.Decode("text/base64", []byte(parts[1]), &decoded)
		if err != nil {
			return nil, fmt.Errorf("failed to decode base64 data: %w", err)
		}
		return decoded, nil
	} else if strings.HasPrefix(imageInput, "http") {
		// URL格式
		return h.downloadImage(imageInput)
	} else {
		// 直接的Base64格式
		// 使用编解码器进行base64解码
		var decoded []byte
		err := h.codecRegistry.Decode("text/base64", []byte(imageInput), &decoded)
		if err != nil {
			return nil, fmt.Errorf("failed to decode base64 data: %w", err)
		}
		return decoded, nil
	}
}

// downloadImage 下载图像
func (h *Handler) downloadImage(url string) ([]byte, error) {
	// 简化实现，实际应该使用HTTP客户端
	return nil, fmt.Errorf("image URL download not implemented yet")
}

// isValidImageInput 检查图像输入是否有效
func isValidImageInput(imageInput string) bool {
	if imageInput == "" {
		return false
	}

	if strings.HasPrefix(imageInput, "data:image/") {
		parts := strings.SplitN(imageInput, ",", 2)
		return len(parts) == 2
	}

	if strings.HasPrefix(imageInput, "http") {
		return true // URL格式暂时认为有效
	}

	// 检查是否为有效的Base64
	_, err := base64.StdEncoding.DecodeString(imageInput)
	return err == nil
}

// generateRequestID 生成请求ID
func generateRequestID() string {
	return fmt.Sprintf("req_%d", time.Now().UnixNano())
}

// 全局变量
var startTime = time.Now()
