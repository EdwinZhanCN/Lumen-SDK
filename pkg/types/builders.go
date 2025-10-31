package types

import (
	"fmt"

	pb "github.com/edwinzhancn/lumen-sdk/proto"
)

type InferRequestBuilder struct {
	req *pb.InferRequest
}

// NewInferRequest creates a new InferRequestBuilder with the given task.
// client will use task to choose the node from catalog and send the request to the selected node
func NewInferRequest(task string) *InferRequestBuilder {
	return &InferRequestBuilder{
		req: &pb.InferRequest{
			Task:        task,
			Meta:        make(map[string]string),
			PayloadMime: "application/json",
		},
	}
}

// WithCorrelationID sets the correlation ID for the request. The system will use generated correlation ID if not provided.
func (b *InferRequestBuilder) WithCorrelationID(id string) *InferRequestBuilder {
	b.req.CorrelationId = id
	return b
}

// WithMeta sets a metadata key-value pair for the request. Please refer to specific metadata definition of the task.
func (b *InferRequestBuilder) WithMeta(key, value string) *InferRequestBuilder {
	if b.req.Meta == nil {
		b.req.Meta = make(map[string]string)
	}
	b.req.Meta[key] = value
	return b
}

func (b *InferRequestBuilder) Build() *pb.InferRequest {
	return b.req
}

// ForEmbedding is a builder method for embedding requests.
// - req is the embedding request payload, use NewEmbeddingRequest to get mimetype automatically.
// - task is the task name provided by node capability for the embedding request. e.g., lumen_clip_embed, lumen_clip_image_embed
// ForEmbedding has no deliverable metadata set as empty string
func (b *InferRequestBuilder) ForEmbedding(req *EmbeddingRequest, task string) *InferRequestBuilder {
	payload := req.Payload
	b.req.Payload = payload
	b.req.PayloadMime = req.PayloadMime
	b.req.Task = task
	return b
}

// ForClassification is a builder method for classification requests.
// - req is the classification request payload, use NewClassificationRequest to get mimetype automatically.
// - task is the task name provided by node capability for the classification request. e.g., lumen_clip_classify, lumen_clip_classify_scene
// ForClassification has no deliverable metadata set as empty string
func (b *InferRequestBuilder) ForClassification(req *ClassificationRequest, task string) *InferRequestBuilder {
	payload := req.Payload
	b.req.Payload = payload
	b.req.PayloadMime = req.PayloadMime
	b.req.Task = task
	return b
}

func (b *InferRequestBuilder) ForFaceDetection(req *FaceRecognitionRequest, task string) *InferRequestBuilder {
	payload := req.Payload
	b.req.Payload = payload
	b.req.PayloadMime = req.PayloadMime
	b.req.Task = task

	// 设置人脸检测相关的元数据
	if req.DetectionConfidenceThreshold > 0 {
		b.WithMeta("detection_confidence_threshold", fmt.Sprintf("%.3f", req.DetectionConfidenceThreshold))
	}
	if req.NmsThreshold > 0 {
		b.WithMeta("nms_threshold", fmt.Sprintf("%.3f", req.NmsThreshold))
	}
	if req.FaceSizeMin > 0 {
		b.WithMeta("face_size_min", fmt.Sprintf("%.1f", req.FaceSizeMin))
	}
	if req.FaceSizeMax > 0 {
		b.WithMeta("face_size_max", fmt.Sprintf("%.1f", req.FaceSizeMax))
	}

	// Max Faces cannot be 0 or samller than -1
	if req.MaxFaces != 0 && req.MaxFaces >= -1 {
		b.WithMeta("max_faces", fmt.Sprintf("%d", req.MaxFaces))
	}

	return b
}
