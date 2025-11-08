package client_test

import (
	"testing"

	"github.com/edwinzhancn/lumen-sdk/pkg/client"
	"github.com/edwinzhancn/lumen-sdk/pkg/config"
)

// TestChunkPayloadDisabled tests chunking when auto-chunking is disabled
func TestChunkPayloadDisabled(t *testing.T) {
	cfg := config.ChunkConfig{
		EnableAuto:    false,
		Threshold:     1024,
		MaxChunkBytes: 256,
	}

	payload := make([]byte, 2048)
	for i := range payload {
		payload[i] = byte(i % 256)
	}

	chunks, err := client.ChunkPayload(payload, cfg)
	if err != nil {
		t.Fatalf("ChunkPayload() error = %v", err)
	}

	if len(chunks) != 1 {
		t.Errorf("Expected 1 chunk when disabled, got %d", len(chunks))
	}

	if len(chunks[0]) != len(payload) {
		t.Errorf("Expected chunk size %d, got %d", len(payload), len(chunks[0]))
	}
}

// TestChunkPayloadBelowThreshold tests chunking when payload is below threshold
func TestChunkPayloadBelowThreshold(t *testing.T) {
	cfg := config.ChunkConfig{
		EnableAuto:    true,
		Threshold:     1024,
		MaxChunkBytes: 256,
	}

	payload := make([]byte, 512) // Below threshold
	for i := range payload {
		payload[i] = byte(i % 256)
	}

	chunks, err := client.ChunkPayload(payload, cfg)
	if err != nil {
		t.Fatalf("ChunkPayload() error = %v", err)
	}

	if len(chunks) != 1 {
		t.Errorf("Expected 1 chunk when below threshold, got %d", len(chunks))
	}

	if len(chunks[0]) != len(payload) {
		t.Errorf("Expected chunk size %d, got %d", len(payload), len(chunks[0]))
	}
}

// TestChunkPayloadExactlyAtThreshold tests chunking when payload equals threshold
func TestChunkPayloadExactlyAtThreshold(t *testing.T) {
	cfg := config.ChunkConfig{
		EnableAuto:    true,
		Threshold:     1024,
		MaxChunkBytes: 256,
	}

	payload := make([]byte, 1024) // Exactly at threshold
	for i := range payload {
		payload[i] = byte(i % 256)
	}

	chunks, err := client.ChunkPayload(payload, cfg)
	if err != nil {
		t.Fatalf("ChunkPayload() error = %v", err)
	}

	if len(chunks) != 1 {
		t.Errorf("Expected 1 chunk when at threshold, got %d", len(chunks))
	}
}

// TestChunkPayloadAboveThreshold tests chunking when payload exceeds threshold
func TestChunkPayloadAboveThreshold(t *testing.T) {
	cfg := config.ChunkConfig{
		EnableAuto:    true,
		Threshold:     1024,
		MaxChunkBytes: 256,
	}

	payload := make([]byte, 2048) // Above threshold
	for i := range payload {
		payload[i] = byte(i % 256)
	}

	chunks, err := client.ChunkPayload(payload, cfg)
	if err != nil {
		t.Fatalf("ChunkPayload() error = %v", err)
	}

	expectedChunks := (len(payload) + cfg.MaxChunkBytes - 1) / cfg.MaxChunkBytes
	if len(chunks) != expectedChunks {
		t.Errorf("Expected %d chunks, got %d", expectedChunks, len(chunks))
	}

	// Verify all chunks except last are MaxChunkBytes
	for i := 0; i < len(chunks)-1; i++ {
		if len(chunks[i]) != cfg.MaxChunkBytes {
			t.Errorf("Chunk %d: expected size %d, got %d", i, cfg.MaxChunkBytes, len(chunks[i]))
		}
	}

	// Verify last chunk size
	expectedLastSize := len(payload) % cfg.MaxChunkBytes
	if expectedLastSize == 0 {
		expectedLastSize = cfg.MaxChunkBytes
	}
	lastChunk := chunks[len(chunks)-1]
	if len(lastChunk) != expectedLastSize {
		t.Errorf("Last chunk: expected size %d, got %d", expectedLastSize, len(lastChunk))
	}
}

// TestChunkPayloadInvalidConfig tests error handling for invalid config
func TestChunkPayloadInvalidConfig(t *testing.T) {
	cfg := config.ChunkConfig{
		EnableAuto:    true,
		Threshold:     1024,
		MaxChunkBytes: 0, // Invalid
	}

	payload := make([]byte, 2048)

	_, err := client.ChunkPayload(payload, cfg)
	if err == nil {
		t.Error("Expected error for invalid MaxChunkBytes, got nil")
	}
}

// TestChunkPayloadEmptyPayload tests chunking with empty payload
func TestChunkPayloadEmptyPayload(t *testing.T) {
	cfg := config.ChunkConfig{
		EnableAuto:    true,
		Threshold:     1024,
		MaxChunkBytes: 256,
	}

	payload := []byte{}

	chunks, err := client.ChunkPayload(payload, cfg)
	if err != nil {
		t.Fatalf("ChunkPayload() error = %v", err)
	}

	if len(chunks) != 1 {
		t.Errorf("Expected 1 chunk for empty payload, got %d", len(chunks))
	}

	if len(chunks[0]) != 0 {
		t.Errorf("Expected empty chunk, got size %d", len(chunks[0]))
	}
}

// TestChunkPayloadDataIntegrity tests that chunked data can be reassembled correctly
func TestChunkPayloadDataIntegrity(t *testing.T) {
	cfg := config.ChunkConfig{
		EnableAuto:    true,
		Threshold:     1024,
		MaxChunkBytes: 256,
	}

	// Create a payload with distinct pattern
	payload := make([]byte, 2048)
	for i := range payload {
		payload[i] = byte(i % 256)
	}

	chunks, err := client.ChunkPayload(payload, cfg)
	if err != nil {
		t.Fatalf("ChunkPayload() error = %v", err)
	}

	// Reassemble chunks
	reassembled := make([]byte, 0, len(payload))
	for _, chunk := range chunks {
		reassembled = append(reassembled, chunk...)
	}

	// Verify data integrity
	if len(reassembled) != len(payload) {
		t.Errorf("Reassembled length %d != original length %d", len(reassembled), len(payload))
	}

	for i := range payload {
		if reassembled[i] != payload[i] {
			t.Errorf("Data mismatch at byte %d: expected %d, got %d", i, payload[i], reassembled[i])
			break
		}
	}
}

// TestChunkPayloadLargePayload tests chunking with a large payload
func TestChunkPayloadLargePayload(t *testing.T) {
	cfg := config.ChunkConfig{
		EnableAuto:    true,
		Threshold:     1024,
		MaxChunkBytes: 512 * 1024, // 512KB chunks
	}

	// Create 5MB payload
	payload := make([]byte, 5*1024*1024)
	for i := 0; i < len(payload); i += 1024 {
		payload[i] = byte(i / 1024)
	}

	chunks, err := client.ChunkPayload(payload, cfg)
	if err != nil {
		t.Fatalf("ChunkPayload() error = %v", err)
	}

	expectedChunks := (len(payload) + cfg.MaxChunkBytes - 1) / cfg.MaxChunkBytes
	if len(chunks) != expectedChunks {
		t.Errorf("Expected %d chunks, got %d", expectedChunks, len(chunks))
	}

	// Verify total size
	totalSize := 0
	for _, chunk := range chunks {
		totalSize += len(chunk)
	}
	if totalSize != len(payload) {
		t.Errorf("Total chunk size %d != payload size %d", totalSize, len(payload))
	}
}

// TestChunkPayloadNegativeMaxChunkBytes tests error handling for negative MaxChunkBytes
func TestChunkPayloadNegativeMaxChunkBytes(t *testing.T) {
	cfg := config.ChunkConfig{
		EnableAuto:    true,
		Threshold:     1024,
		MaxChunkBytes: -1,
	}

	payload := make([]byte, 2048)

	_, err := client.ChunkPayload(payload, cfg)
	if err == nil {
		t.Error("Expected error for negative MaxChunkBytes, got nil")
	}
}
