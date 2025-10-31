package types

import (
	"encoding/json"
	"fmt"

	pb "github.com/edwinzhancn/lumen-sdk/proto"
)

// InferResponseParser 响应解析器
type InferResponseParser struct {
	resp *pb.InferResponse
}

func ParseInferResponse(resp *pb.InferResponse) *InferResponseParser {
	return &InferResponseParser{resp: resp}
}

func (p *InferResponseParser) AsFaceResponse() (*FaceV1, error) {
	if p.resp.ResultMime != "application/json;schema=face_v1" {
		return nil, fmt.Errorf("unexpected response type: %s", p.resp.ResultMime)
	}

	var result FaceV1
	if err := json.Unmarshal(p.resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse detection response: %w", err)
	}
	return &result, nil
}

func (p *InferResponseParser) AsEmbeddingResponse() (*EmbeddingV1, error) {
	if p.resp.ResultMime != "application/json;schema=embedding_v1" {
		return nil, fmt.Errorf("unexpected response type: %s", p.resp.ResultMime)
	}

	var result EmbeddingV1
	if err := json.Unmarshal(p.resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse embedding response: %w", err)
	}
	return &result, nil
}

func (p *InferResponseParser) AsClassificationResponse() (*LabelsV1, error) {
	if p.resp.ResultMime != "application/json;schema=labels_v1" {
		return nil, fmt.Errorf("unexpected response type: %s", p.resp.ResultMime)
	}

	var result LabelsV1
	if err := json.Unmarshal(p.resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse classification response: %w", err)
	}
	return &result, nil
}

// 原始响应访问
func (p *InferResponseParser) Raw() *pb.InferResponse {
	return p.resp
}
