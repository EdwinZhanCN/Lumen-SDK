package client

import (
	"errors"

	"github.com/edwinzhancn/lumen-sdk/pkg/config"
)

// ChunkPayload 按配置把 payload 切成 chunk 列表。
// 返回的切片顺序就是 seq 从 0..n-1
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
