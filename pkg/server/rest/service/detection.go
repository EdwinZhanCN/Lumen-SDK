package service

import (
	"context"

	"github.com/edwinzhancn/lumen-sdk/pkg/client"
	"github.com/edwinzhancn/lumen-sdk/pkg/types"
	pb "github.com/edwinzhancn/lumen-sdk/proto"
)

type detectionService struct {
	client *client.LumenClient
}

func NewDetectionService(client *client.LumenClient) DetectionService {
	return &detectionService{
		client: client,
	}
}

func (s *detectionService) GetFaceDetection(ctx context.Context, req *types.FaceRecognitionRequest, task string) (*pb.InferResponse, error) {
	inferReq := types.NewInferRequest(task).ForFaceDetection(req, task).Build()
	resp, err := s.client.Infer(ctx, inferReq)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (s *detectionService) GetFaceRecognition(ctx context.Context, req *types.FaceRecognitionRequest, task string) (*pb.InferResponse, error) {
	return s.client.Infer(ctx, types.NewInferRequest(task).ForFaceDetection(req, task).Build())
}

// GetFaceDetectionStream returns a channel of InferResponse for streaming detection results.
// It leverages the client's InferStream which already supports chunking and streaming.
func (s *detectionService) GetFaceDetectionStream(ctx context.Context, req *types.FaceRecognitionRequest, task string) (<-chan *pb.InferResponse, error) {
	inferReq := types.NewInferRequest(task).ForFaceDetection(req, task).Build()
	return s.client.InferStream(ctx, inferReq)
}

// GetFaceRecognitionStream returns a channel of InferResponse for streaming face recognition results.
func (s *detectionService) GetFaceRecognitionStream(ctx context.Context, req *types.FaceRecognitionRequest, task string) (<-chan *pb.InferResponse, error) {
	inferReq := types.NewInferRequest(task).ForFaceDetection(req, task).Build()
	return s.client.InferStream(ctx, inferReq)
}
