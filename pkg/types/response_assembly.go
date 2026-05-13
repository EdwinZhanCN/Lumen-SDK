package types

import (
	"fmt"

	pb "github.com/edwinzhancn/lumen-sdk/proto"
)

var outputContractMetaKeys = []string{
	MetaOutputKind,
	MetaOutputTensorDType,
	MetaOutputTensorShape,
	MetaOutputTensorLayout,
	MetaOutputTensorFormat,
	MetaOutputTensorByteOrder,
	MetaModelID,
	MetaModelVersion,
}

// AssembleInferResponses returns the final semantic response from a response stream.
// Responses with Total > 1 are treated as transport chunks and strictly reassembled.
// Legacy or semantic streaming responses without Total > 1 return the final response,
// or the last response when no final marker is present.
func AssembleInferResponses(responses []*pb.InferResponse) (*pb.InferResponse, error) {
	if len(responses) == 0 {
		return nil, fmt.Errorf("no responses to assemble")
	}

	var total uint64
	for i, resp := range responses {
		if resp == nil {
			return nil, fmt.Errorf("response at index %d is nil", i)
		}
		if resp.Total > 1 {
			if total == 0 {
				total = resp.Total
				continue
			}
			if resp.Total != total {
				return nil, fmt.Errorf("inconsistent response chunk total: got %d, want %d", resp.Total, total)
			}
		}
	}

	if total == 0 {
		return finalOrLastResponse(responses), nil
	}

	maxInt := int(^uint(0) >> 1)
	if total > uint64(maxInt) {
		return nil, fmt.Errorf("response chunk total too large: %d", total)
	}
	if uint64(len(responses)) != total {
		return nil, fmt.Errorf("expected %d response chunks, got %d", total, len(responses))
	}

	chunks := make([]*pb.InferResponse, int(total))
	for _, resp := range responses {
		if resp.Total != total {
			return nil, fmt.Errorf("all response chunks must declare total %d", total)
		}
		if resp.Seq >= total {
			return nil, fmt.Errorf("response chunk seq %d out of range for total %d", resp.Seq, total)
		}
		if chunks[int(resp.Seq)] != nil {
			return nil, fmt.Errorf("duplicate response chunk seq %d", resp.Seq)
		}
		chunks[int(resp.Seq)] = resp
	}

	first := chunks[0]
	if first == nil {
		return nil, fmt.Errorf("missing response chunk seq 0")
	}

	var result []byte
	var expectedOffset uint64
	for seq, chunk := range chunks {
		if chunk == nil {
			return nil, fmt.Errorf("missing response chunk seq %d", seq)
		}
		if chunk.Offset != expectedOffset {
			return nil, fmt.Errorf("response chunk seq %d offset mismatch: got %d, want %d", seq, chunk.Offset, expectedOffset)
		}
		if seq == len(chunks)-1 {
			if !chunk.IsFinal {
				return nil, fmt.Errorf("final response chunk seq %d must be marked final", seq)
			}
		} else if chunk.IsFinal {
			return nil, fmt.Errorf("non-terminal response chunk seq %d must not be marked final", seq)
		}
		if err := validateChunkContract(first, chunk, seq); err != nil {
			return nil, err
		}

		result = append(result, chunk.Result...)
		expectedOffset += uint64(len(chunk.Result))
	}

	finalChunk := chunks[len(chunks)-1]
	return &pb.InferResponse{
		CorrelationId: finalChunk.CorrelationId,
		IsFinal:       true,
		Result:        result,
		Meta:          cloneStringMap(finalChunk.Meta),
		Error:         clonePBError(finalChunk.Error),
		Seq:           0,
		Total:         1,
		Offset:        0,
		ResultMime:    finalChunk.ResultMime,
		ResultSchema:  finalChunk.ResultSchema,
	}, nil
}

func finalOrLastResponse(responses []*pb.InferResponse) *pb.InferResponse {
	for i := len(responses) - 1; i >= 0; i-- {
		if responses[i].IsFinal {
			return responses[i]
		}
	}
	return responses[len(responses)-1]
}

func validateChunkContract(first, current *pb.InferResponse, seq int) error {
	if current.CorrelationId != first.CorrelationId {
		return fmt.Errorf("response chunk seq %d correlation_id mismatch", seq)
	}
	if current.ResultMime != first.ResultMime {
		return fmt.Errorf("response chunk seq %d result_mime mismatch", seq)
	}
	if current.ResultSchema != first.ResultSchema {
		return fmt.Errorf("response chunk seq %d result_schema mismatch", seq)
	}
	for _, key := range outputContractMetaKeys {
		if current.Meta[key] != first.Meta[key] {
			return fmt.Errorf("response chunk seq %d metadata %s mismatch", seq, key)
		}
	}
	return nil
}

func cloneStringMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func clonePBError(in *pb.Error) *pb.Error {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}
