package types_test

import (
	"strings"
	"testing"

	"github.com/edwinzhancn/lumen-sdk/pkg/types"
	pb "github.com/edwinzhancn/lumen-sdk/proto"
)

func TestAssembleInferResponsesOutOfOrderChunks(t *testing.T) {
	assembled, err := types.AssembleInferResponses([]*pb.InferResponse{
		responseChunk(1, 2, 2, "llo", true),
		responseChunk(0, 2, 0, "he", false),
	})
	if err != nil {
		t.Fatalf("AssembleInferResponses() error = %v", err)
	}
	if string(assembled.Result) != "hello" {
		t.Fatalf("expected assembled result hello, got %q", string(assembled.Result))
	}
	if !assembled.IsFinal || assembled.Seq != 0 || assembled.Total != 1 || assembled.Offset != 0 {
		t.Fatalf("unexpected assembled chunk markers: seq=%d total=%d offset=%d final=%v",
			assembled.Seq, assembled.Total, assembled.Offset, assembled.IsFinal)
	}
}

func TestAssembleInferResponsesMissingChunk(t *testing.T) {
	_, err := types.AssembleInferResponses([]*pb.InferResponse{
		responseChunk(0, 2, 0, "he", false),
	})
	if err == nil || !strings.Contains(err.Error(), "expected 2 response chunks") {
		t.Fatalf("expected missing chunk error, got %v", err)
	}
}

func TestAssembleInferResponsesDuplicateSeq(t *testing.T) {
	_, err := types.AssembleInferResponses([]*pb.InferResponse{
		responseChunk(0, 2, 0, "he", false),
		responseChunk(0, 2, 2, "llo", true),
	})
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("expected duplicate seq error, got %v", err)
	}
}

func TestAssembleInferResponsesBadOffset(t *testing.T) {
	_, err := types.AssembleInferResponses([]*pb.InferResponse{
		responseChunk(0, 2, 0, "he", false),
		responseChunk(1, 2, 3, "llo", true),
	})
	if err == nil || !strings.Contains(err.Error(), "offset mismatch") {
		t.Fatalf("expected offset mismatch, got %v", err)
	}
}

func TestAssembleInferResponsesInconsistentMime(t *testing.T) {
	second := responseChunk(1, 2, 2, "llo", true)
	second.ResultMime = "text/plain"

	_, err := types.AssembleInferResponses([]*pb.InferResponse{
		responseChunk(0, 2, 0, "he", false),
		second,
	})
	if err == nil || !strings.Contains(err.Error(), "result_mime mismatch") {
		t.Fatalf("expected result_mime mismatch, got %v", err)
	}
}

func TestAssembleInferResponsesInconsistentSchema(t *testing.T) {
	second := responseChunk(1, 2, 2, "llo", true)
	second.ResultSchema = "other_schema"

	_, err := types.AssembleInferResponses([]*pb.InferResponse{
		responseChunk(0, 2, 0, "he", false),
		second,
	})
	if err == nil || !strings.Contains(err.Error(), "result_schema mismatch") {
		t.Fatalf("expected result_schema mismatch, got %v", err)
	}
}

func TestAssembleInferResponsesInconsistentOutputMeta(t *testing.T) {
	second := responseChunk(1, 2, 2, "llo", true)
	second.Meta[types.MetaOutputKind] = types.OutputKindTensor

	_, err := types.AssembleInferResponses([]*pb.InferResponse{
		responseChunk(0, 2, 0, "he", false),
		second,
	})
	if err == nil || !strings.Contains(err.Error(), types.MetaOutputKind) {
		t.Fatalf("expected output metadata mismatch, got %v", err)
	}
}

func TestAssembleInferResponsesFinalNotLast(t *testing.T) {
	_, err := types.AssembleInferResponses([]*pb.InferResponse{
		responseChunk(0, 2, 0, "he", true),
		responseChunk(1, 2, 2, "llo", false),
	})
	if err == nil || !strings.Contains(err.Error(), "non-terminal") {
		t.Fatalf("expected non-terminal final error, got %v", err)
	}
}

func TestAssembleInferResponsesFinalChunkMustBeFinal(t *testing.T) {
	_, err := types.AssembleInferResponses([]*pb.InferResponse{
		responseChunk(0, 2, 0, "he", false),
		responseChunk(1, 2, 2, "llo", false),
	})
	if err == nil || !strings.Contains(err.Error(), "must be marked final") {
		t.Fatalf("expected final chunk marker error, got %v", err)
	}
}

func responseChunk(seq, total, offset uint64, result string, final bool) *pb.InferResponse {
	return &pb.InferResponse{
		CorrelationId: "corr-1",
		IsFinal:       final,
		Result:        []byte(result),
		Meta: map[string]string{
			types.MetaOutputKind: types.OutputKindRaw,
		},
		Seq:          seq,
		Total:        total,
		Offset:       offset,
		ResultMime:   types.DefaultTensorMIME,
		ResultSchema: "",
	}
}
