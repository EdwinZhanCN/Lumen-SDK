//go:generate go run ../cmd/genconst -input=interface.go -output=../constants_gen.go
package service

import (
	"context"

	"github.com/edwinzhancn/lumen-sdk/pkg/types"
	pb "github.com/edwinzhancn/lumen-sdk/proto"
)

// EmbeddingService 提供 embedding 相关的同步与流式接口
type EmbeddingService interface {
	GetEmbedding(ctx context.Context, req *types.EmbeddingRequest, task string) (*pb.InferResponse, error)
	// GetEmbeddingStream 返回一个响应通道，适用于增量/流式场景
	GetEmbeddingStream(ctx context.Context, req *types.EmbeddingRequest, task string) (<-chan *pb.InferResponse, error)
}

// DetectionService 提供人脸检测/识别的同步与流式接口
type DetectionService interface {
	GetFaceDetection(ctx context.Context, req *types.FaceRecognitionRequest, task string) (*pb.InferResponse, error)
	// 流式版本，返回响应流
	GetFaceDetectionStream(ctx context.Context, req *types.FaceRecognitionRequest, task string) (<-chan *pb.InferResponse, error)

	GetFaceRecognition(ctx context.Context, req *types.FaceRecognitionRequest, task string) (*pb.InferResponse, error)
	// 流式版本，返回响应流
	GetFaceRecognitionStream(ctx context.Context, req *types.FaceRecognitionRequest, task string) (<-chan *pb.InferResponse, error)
}

// ClassificationService 提供分类的同步与流式接口
type ClassificationService interface {
	GetClassification(ctx context.Context, req *types.ClassificationRequest, task string) (*pb.InferResponse, error)
	// 流式版本，返回响应流
	GetClassificationStream(ctx context.Context, req *types.ClassificationRequest, task string) (<-chan *pb.InferResponse, error)
}
