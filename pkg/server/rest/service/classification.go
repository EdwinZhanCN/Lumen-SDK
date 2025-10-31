package service

import (
	"context"

	"github.com/edwinzhancn/lumen-sdk/pkg/client"
	"github.com/edwinzhancn/lumen-sdk/pkg/types"
	pb "github.com/edwinzhancn/lumen-sdk/proto"
)

type classificationService struct {
	client *client.LumenClient
}

func NewClassificationService(client *client.LumenClient) ClassificationService {
	return &classificationService{client: client}
}

func (s *classificationService) GetClassification(ctx context.Context, req *types.ClassificationRequest, task string) (*pb.InferResponse, error) {
	inferReq := types.NewInferRequest(task).
		ForClassification(req, task).
		Build()

	resp, err := s.client.Infer(ctx, inferReq)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// GetClassificationStream returns a channel of InferResponse for streaming classification results.
// It leverages the client's InferStream which already supports chunking and streaming.
func (s *classificationService) GetClassificationStream(ctx context.Context, req *types.ClassificationRequest, task string) (<-chan *pb.InferResponse, error) {
	inferReq := types.NewInferRequest(task).
		ForClassification(req, task).
		Build()
	return s.client.InferStream(ctx, inferReq)
}
