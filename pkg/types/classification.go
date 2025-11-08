package types

import (
	"fmt"
	"sort"

	"github.com/gabriel-vasile/mimetype"
)

type LabelsV1 struct {
	Labels  []Label `json:"labels" example:"[{\"label\": \"cat\", \"score\": 0.9}, {\"label\": \"dog\", \"score\": 0.1}]"`
	ModelID string  `json:"model_id" example:"embedding_model_1"`
}

type Label struct {
	Label string  `json:"label"`
	Score float32 `json:"score"`
}

func (l LabelsV1) TopK(k int) []Label {
	if k > len(l.Labels) {
		k = len(l.Labels)
	}
	sort.Slice(l.Labels, func(i, j int) bool {
		return l.Labels[i].Score > l.Labels[j].Score
	})
	return l.Labels[:k]
}

type ClassificationRequest struct {
	Payload     []byte `json:"payload"`
	PayloadMime string `json:"payload_mime_type"`
}

// NewClassificationRequest creates a new ClassificationRequest instance.
// Accepts MimeTypes from SupportedImageMimeTypes
func NewClassificationRequest(payload []byte) (*ClassificationRequest, error) {
	mime := mimetype.Detect(payload)
	mimeString := mime.String()

	// Check if detected MIME type matches any supported image type
	if mimetype.EqualsAny(mimeString, SupportedImageMimeTypes...) {
		return &ClassificationRequest{
			Payload:     payload,
			PayloadMime: mimeString,
		}, nil
	}

	return nil, fmt.Errorf("unsupported payload type: %s", mimeString)
}
