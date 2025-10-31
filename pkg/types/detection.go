package types

import (
	"fmt"

	"github.com/gabriel-vasile/mimetype"
)

type FaceV1 struct {
	Faces   []Face `json:"faces"`
	Count   int    `json:"count"`
	ModelID string `json:"model_id"`
}

type Face struct {
	BBox       []float32 `json:"bbox"` //  [x, y, w, h]
	Confidence float32   `json:"confidence"`
	Landmarks  []float32 `json:"landmarks,omitempty"`
	Embedding  []float32 `json:"embedding,omitempty"`
}

type FaceRecognitionRequest struct {
	Payload                      []byte  `json:"payload"`
	PayloadMime                  string  `json:"payload_mime_type"`
	DetectionConfidenceThreshold float32 `json:"detection_confidence_threshold,omitempty"`
	NmsThreshold                 float32 `json:"nms_threshold,omitempty"`
	FaceSizeMin                  float32 `json:"face_size_min,omitempty"`
	FaceSizeMax                  float32 `json:"face_size_max,omitempty"`
	MaxFaces                     int     `json:"max_faces,omitempty"` // -1 means no limit
}

// FaceRecognitionOption 定义人脸检测请求的选项函数类型
type FaceRecognitionOption func(*FaceRecognitionRequest)

// WithDetectionConfidenceThreshold 设置检测置信度阈值
func WithDetectionConfidenceThreshold(threshold float32) FaceRecognitionOption {
	return func(req *FaceRecognitionRequest) {
		req.DetectionConfidenceThreshold = threshold
	}
}

// WithNmsThreshold 设置 NMS 阈值
func WithNmsThreshold(threshold float32) FaceRecognitionOption {
	return func(req *FaceRecognitionRequest) {
		req.NmsThreshold = threshold
	}
}

// WithFaceSizeMin 设置最小人脸尺寸
func WithFaceSizeMin(size float32) FaceRecognitionOption {
	return func(req *FaceRecognitionRequest) {
		req.FaceSizeMin = size
	}
}

// WithFaceSizeMax 设置最大人脸尺寸
func WithFaceSizeMax(size float32) FaceRecognitionOption {
	return func(req *FaceRecognitionRequest) {
		req.FaceSizeMax = size
	}
}

// WithMaxFaces 设置最大人脸数量
func WithMaxFaces(maxFaces int) FaceRecognitionOption {
	return func(req *FaceRecognitionRequest) {
		req.MaxFaces = maxFaces
	}
}

// NewFaceRecognitionRequest 创建新的人脸检测请求
func NewFaceRecognitionRequest(payload []byte, opts ...FaceRecognitionOption) (*FaceRecognitionRequest, error) {
	mime := mimetype.Detect(payload)
	if mimetype.EqualsAny(mime.String(), SupportedImageMimeTypes) {
		payloadMime := mime.String()
		req := &FaceRecognitionRequest{
			Payload:     payload,
			PayloadMime: payloadMime,
		}

		// 应用所有选项
		for _, opt := range opts {
			opt(req)
		}

		return req, nil
	}
	return nil, fmt.Errorf("unsupported payload type: %s", mime.String())
}
