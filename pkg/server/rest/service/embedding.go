package service

import (
	"context"

	"github.com/edwinzhancn/lumen-sdk/pkg/client"
	"github.com/edwinzhancn/lumen-sdk/pkg/types"
	pb "github.com/edwinzhancn/lumen-sdk/proto"
)

type embeddingService struct {
	client *client.LumenClient
}

func NewEmbeddingService(client *client.LumenClient) EmbeddingService {
	return &embeddingService{
		client: client,
	}
}

func (s *embeddingService) GetEmbedding(ctx context.Context, req *types.EmbeddingRequest, task string) (*pb.InferResponse, error) {
	inferReq := types.NewInferRequest(task).
		ForEmbedding(req, task).
		Build()
	resp, err := s.client.Infer(ctx, inferReq)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// GetEmbeddingStream returns a channel of InferResponse for streaming / incremental results.
// It leverages the client's InferStream which already supports chunking and streaming.
func (s *embeddingService) GetEmbeddingStream(ctx context.Context, req *types.EmbeddingRequest, task string) (<-chan *pb.InferResponse, error) {
	inferReq := types.NewInferRequest(task).
		ForEmbedding(req, task).
		Build()
	return s.client.InferStream(ctx, inferReq)
}
