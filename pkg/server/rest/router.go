package rest

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/edwinzhancn/lumen-sdk/pkg/client"
	"github.com/edwinzhancn/lumen-sdk/pkg/server/rest/service"
	"github.com/edwinzhancn/lumen-sdk/pkg/types"
)

// ServiceRouter handles routing between different ML services
type ServiceRouter struct {
	detectionService      service.DetectionService
	classificationService service.ClassificationService
	embeddingService      service.EmbeddingService

	// dispatch map: service string -> handler
	handlers map[string]func(ctx context.Context, req RESTInferRequest) (interface{}, error)
}

// NewServiceRouter creates a new ServiceRouter instance
func NewServiceRouter(client *client.LumenClient) *ServiceRouter {
	r := &ServiceRouter{
		detectionService:      service.NewDetectionService(client),
		classificationService: service.NewClassificationService(client),
		embeddingService:      service.NewEmbeddingService(client),
		handlers:              make(map[string]func(ctx context.Context, req RESTInferRequest) (interface{}, error)),
	}

	// build handlers - closures capture service instances
	r.handlers[ServiceEmbedding] = func(ctx context.Context, req RESTInferRequest) (interface{}, error) {
		newReq, err := types.NewEmbeddingRequest(req.Payload)
		if err != nil {
			return nil, err
		}
		return r.embeddingService.GetEmbedding(ctx, newReq, req.Task)
	}

	// Stream variant for embedding
	r.handlers[ServiceEmbeddingStream] = func(ctx context.Context, req RESTInferRequest) (interface{}, error) {
		newReq, err := types.NewEmbeddingRequest(req.Payload)
		if err != nil {
			return nil, err
		}
		return r.embeddingService.GetEmbeddingStream(ctx, newReq, req.Task)
	}

	r.handlers[ServiceClassification] = func(ctx context.Context, req RESTInferRequest) (interface{}, error) {
		newReq, err := types.NewClassificationRequest(req.Payload)
		if err != nil {
			return nil, err
		}
		return r.classificationService.GetClassification(ctx, newReq, req.Task)
	}

	// Stream variant for classification
	r.handlers[ServiceClassificationStream] = func(ctx context.Context, req RESTInferRequest) (interface{}, error) {
		newReq, err := types.NewClassificationRequest(req.Payload)
		if err != nil {
			return nil, err
		}
		return r.classificationService.GetClassificationStream(ctx, newReq, req.Task)
	}

	// face detection and face recognition map to different methods
	r.handlers[ServiceFaceDetection] = func(ctx context.Context, req RESTInferRequest) (interface{}, error) {
		opts := buildFaceRecognitionOptions(req.Metadata)
		newReq, err := types.NewFaceRecognitionRequest(req.Payload, opts...)
		if err != nil {
			return nil, err
		}
		return r.detectionService.GetFaceDetection(ctx, newReq, req.Task)
	}

	// Stream variant for face detection
	r.handlers[ServiceFaceDetectionStream] = func(ctx context.Context, req RESTInferRequest) (interface{}, error) {
		opts := buildFaceRecognitionOptions(req.Metadata)
		newReq, err := types.NewFaceRecognitionRequest(req.Payload, opts...)
		if err != nil {
			return nil, err
		}
		return r.detectionService.GetFaceDetectionStream(ctx, newReq, req.Task)
	}

	r.handlers[ServiceFaceRecognition] = func(ctx context.Context, req RESTInferRequest) (interface{}, error) {
		opts := buildFaceRecognitionOptions(req.Metadata)
		newReq, err := types.NewFaceRecognitionRequest(req.Payload, opts...)
		if err != nil {
			return nil, err
		}
		return r.detectionService.GetFaceRecognition(ctx, newReq, req.Task)
	}

	// Stream variant for face recognition
	r.handlers[ServiceFaceRecognitionStream] = func(ctx context.Context, req RESTInferRequest) (interface{}, error) {
		opts := buildFaceRecognitionOptions(req.Metadata)
		newReq, err := types.NewFaceRecognitionRequest(req.Payload, opts...)
		if err != nil {
			return nil, err
		}
		return r.detectionService.GetFaceRecognitionStream(ctx, newReq, req.Task)
	}

	return r
}

// RouteRequest routes the request using the dispatch map
func (r *ServiceRouter) RouteRequest(ctx context.Context, req RESTInferRequest) (interface{}, error) {
	// normalize key (可选)
	key := strings.ToLower(strings.TrimSpace(req.Service))
	if handler, ok := r.handlers[key]; ok {
		return handler(ctx, req)
	}
	return nil, fmt.Errorf("unsupported service: %s", req.Service)
}

// buildFaceRecognitionOptions 与之前示例类似
func buildFaceRecognitionOptions(metadata map[string]string) []types.FaceRecognitionOption {
	var opts []types.FaceRecognitionOption
	if metadata == nil {
		return opts
	}

	if v, ok := metadata["detection_confidence_threshold"]; ok && v != "" {
		if f, err := strconv.ParseFloat(v, 32); err == nil {
			opts = append(opts, types.WithDetectionConfidenceThreshold(float32(f)))
		}
	}
	if v, ok := metadata["nms_threshold"]; ok && v != "" {
		if f, err := strconv.ParseFloat(v, 32); err == nil {
			opts = append(opts, types.WithNmsThreshold(float32(f)))
		}
	}
	if v, ok := metadata["face_size_min"]; ok && v != "" {
		if f, err := strconv.ParseFloat(v, 32); err == nil {
			opts = append(opts, types.WithFaceSizeMin(float32(f)))
		}
	}
	if v, ok := metadata["face_size_max"]; ok && v != "" {
		if f, err := strconv.ParseFloat(v, 32); err == nil {
			opts = append(opts, types.WithFaceSizeMax(float32(f)))
		}
	}
	if v, ok := metadata["max_faces"]; ok && v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			opts = append(opts, types.WithMaxFaces(i))
		}
	}
	return opts
}
