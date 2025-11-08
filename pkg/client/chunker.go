package client

import (
	"errors"

	"github.com/edwinzhancn/lumen-sdk/pkg/config"
)

// ChunkPayload splits a large payload into smaller chunks based on configuration.
//
// This function enables efficient transmission of large data (images, videos, documents)
// over gRPC by breaking them into manageable pieces. Chunking prevents:
//   - gRPC message size limit errors
//   - Network timeout issues
//   - Memory exhaustion on receiving side
//
// Behavior:
//   - If EnableAuto is false, returns the payload as a single chunk
//   - If payload size ≤ Threshold, returns the payload as a single chunk
//   - Otherwise, splits payload into chunks of MaxChunkBytes size
//
// The returned chunks are ordered sequentially (seq 0..n-1) and should be
// sent in order to maintain data integrity.
//
// Parameters:
//   - payload: The data to potentially chunk
//   - cfg: Chunking configuration (auto enable, threshold, max chunk size)
//
// Returns:
//   - [][]byte: Array of payload chunks (may be single element if no chunking)
//   - error: Non-nil if configuration is invalid (e.g., MaxChunkBytes <= 0)
//
// Role in project: Enables reliable handling of large payloads in distributed
// ML inference. Critical for operations like video analysis, high-res image
// processing, and large document embedding.
//
// Example:
//
//	cfg := config.ChunkConfig{
//	    EnableAuto:    true,
//	    Threshold:     1 << 20,    // 1MB
//	    MaxChunkBytes: 256 * 1024, // 256KB chunks
//	}
//
//	largeImage, _ := os.ReadFile("high_res.jpg") // 5MB image
//	chunks, err := client.ChunkPayload(largeImage, cfg)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Split into %d chunks\n", len(chunks))
//	// Output: Split into 20 chunks
func ChunkPayload(payload []byte, cfg config.ChunkConfig) ([][]byte, error) {
	if !cfg.EnableAuto {
		return [][]byte{payload}, nil
	}
	if cfg.MaxChunkBytes <= 0 {
		return nil, errors.New("invalid MaxChunkBytes")
	}
	if len(payload) <= cfg.Threshold {
		return [][]byte{payload}, nil
	}
	var chunks [][]byte
	for off := 0; off < len(payload); off += cfg.MaxChunkBytes {
		end := off + cfg.MaxChunkBytes
		if end > len(payload) {
			end = len(payload)
		}
		// 注意：为了减少内存复制，你可以使用 payload[off:end] 的切片（要注意生命周期）
		chunks = append(chunks, payload[off:end])
	}
	return chunks, nil
}
