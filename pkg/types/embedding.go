package types

import (
	"encoding/json"
	"fmt"
	"math"
)

// EmbeddingVector 嵌入向量
type EmbeddingVector []float32

// NewEmbeddingVector 创建新的嵌入向量
func NewEmbeddingVector(data []float32) EmbeddingVector {
	return EmbeddingVector(data)
}

// NewEmbeddingVectorFromFloat64 从float64切片创建嵌入向量
func NewEmbeddingVectorFromFloat64(data []float64) EmbeddingVector {
	vec := make(EmbeddingVector, len(data))
	for i, v := range data {
		vec[i] = float32(v)
	}
	return vec
}

// Dim 返回向量维度
func (e EmbeddingVector) Dim() int {
	return len(e)
}

// IsEmpty 检查向量是否为空
func (e EmbeddingVector) IsEmpty() bool {
	return len(e) == 0
}

// Normalize 标准化向量（L2标准化）
func (e EmbeddingVector) Normalize() EmbeddingVector {
	if e.IsEmpty() {
		return e
	}

	norm := float32(0.0)
	for _, v := range e {
		norm += v * v
	}
	norm = float32(math.Sqrt(float64(norm)))

	if norm == 0.0 {
		return e
	}

	normalized := make(EmbeddingVector, len(e))
	for i, v := range e {
		normalized[i] = v / norm
	}

	return normalized
}

// Magnitude 计算向量模长
func (e EmbeddingVector) Magnitude() float32 {
	if e.IsEmpty() {
		return 0.0
	}

	sum := float32(0.0)
	for _, v := range e {
		sum += v * v
	}

	return float32(math.Sqrt(float64(sum)))
}

// Dot 计算点积
func (e EmbeddingVector) Dot(other EmbeddingVector) (float32, error) {
	if len(e) != len(other) {
		return 0.0, fmt.Errorf("vector dimensions mismatch: %d vs %d", len(e), len(other))
	}

	if e.IsEmpty() {
		return 0.0, nil
	}

	sum := float32(0.0)
	for i, v := range e {
		sum += v * other[i]
	}

	return sum, nil
}

// CosineSimilarity 计算余弦相似度
func (e EmbeddingVector) CosineSimilarity(other EmbeddingVector) (float32, error) {
	if len(e) != len(other) {
		return 0.0, fmt.Errorf("vector dimensions mismatch: %d vs %d", len(e), len(other))
	}

	if e.IsEmpty() || other.IsEmpty() {
		return 0.0, nil
	}

	// 计算点积
	dot, err := e.Dot(other)
	if err != nil {
		return 0.0, err
	}

	// 计算模长
	mag1 := e.Magnitude()
	mag2 := other.Magnitude()

	if mag1 == 0.0 || mag2 == 0.0 {
		return 0.0, nil
	}

	return dot / (mag1 * mag2), nil
}

// EuclideanDistance 计算欧几里得距离
func (e EmbeddingVector) EuclideanDistance(other EmbeddingVector) (float32, error) {
	if len(e) != len(other) {
		return 0.0, fmt.Errorf("vector dimensions mismatch: %d vs %d", len(e), len(other))
	}

	if e.IsEmpty() {
		return 0.0, nil
	}

	sum := float32(0.0)
	for i, v := range e {
		diff := v - other[i]
		sum += diff * diff
	}

	return float32(math.Sqrt(float64(sum))), nil
}

// ManhattanDistance 计算曼哈顿距离
func (e EmbeddingVector) ManhattanDistance(other EmbeddingVector) (float32, error) {
	if len(e) != len(other) {
		return 0.0, fmt.Errorf("vector dimensions mismatch: %d vs %d", len(e), len(other))
	}

	if e.IsEmpty() {
		return 0.0, nil
	}

	sum := float32(0.0)
	for i, v := range e {
		diff := v - other[i]
		if diff < 0 {
			sum -= diff
		} else {
			sum += diff
		}
	}

	return sum, nil
}

// ToFloat64 转换为float64切片
func (e EmbeddingVector) ToFloat64() []float64 {
	result := make([]float64, len(e))
	for i, v := range e {
		result[i] = float64(v)
	}
	return result
}

// Clone 克隆向量
func (e EmbeddingVector) Clone() EmbeddingVector {
	if e.IsEmpty() {
		return EmbeddingVector{}
	}

	clone := make(EmbeddingVector, len(e))
	copy(clone, e)
	return clone
}

// Scale 向量缩放
func (e EmbeddingVector) Scale(scalar float32) EmbeddingVector {
	if e.IsEmpty() {
		return e
	}

	scaled := make(EmbeddingVector, len(e))
	for i, v := range e {
		scaled[i] = v * scalar
	}

	return scaled
}

// Add 向量加法
func (e EmbeddingVector) Add(other EmbeddingVector) (EmbeddingVector, error) {
	if len(e) != len(other) {
		return nil, fmt.Errorf("vector dimensions mismatch: %d vs %d", len(e), len(other))
	}

	if e.IsEmpty() {
		return other.Clone(), nil
	}

	result := make(EmbeddingVector, len(e))
	for i, v := range e {
		result[i] = v + other[i]
	}

	return result, nil
}

// Subtract 向量减法
func (e EmbeddingVector) Subtract(other EmbeddingVector) (EmbeddingVector, error) {
	if len(e) != len(other) {
		return nil, fmt.Errorf("vector dimensions mismatch: %d vs %d", len(e), len(other))
	}

	if e.IsEmpty() {
		return other.Scale(-1.0), nil
	}

	result := make(EmbeddingVector, len(e))
	for i, v := range e {
		result[i] = v - other[i]
	}

	return result, nil
}

// EmbeddingRequest 嵌入请求
type EmbeddingRequest struct {
	Text      string            `json:"text"`               // 要编码的文本
	Image     []byte            `json:"image,omitempty"`    // 要编码的图像数据
	MimeType  string            `json:"mime_type"`          // 数据MIME类型
	ModelID   string            `json:"model_id"`           // 模型ID
	Normalize bool              `json:"normalize"`          // 是否标准化结果
	Metadata  map[string]string `json:"metadata,omitempty"` // 元数据
}

// EmbeddingResponse 嵌入响应
type EmbeddingResponse struct {
	Vector      EmbeddingVector   `json:"vector"`             // 嵌入向量
	Dimension   int               `json:"dimension"`          // 向量维度
	ModelID     string            `json:"model_id"`           // 使用的模型ID
	Confidence  float32           `json:"confidence"`         // 置信度
	ProcessTime float64           `json:"process_time_ms"`    // 处理时间(毫秒)
	Metadata    map[string]string `json:"metadata,omitempty"` // 响应元数据
}

// BatchEmbeddingRequest 批量嵌入请求
type BatchEmbeddingRequest struct {
	Requests       []EmbeddingRequest `json:"requests"`        // 嵌入请求列表
	MaxConcurrency int                `json:"max_concurrency"` // 最大并发数
	BatchSize      int                `json:"batch_size"`      // 批处理大小
}

// BatchEmbeddingResponse 批量嵌入响应
type BatchEmbeddingResponse struct {
	Results      []EmbeddingResponse `json:"results"`         // 嵌入结果列表
	TotalCount   int                 `json:"total_count"`     // 总数量
	SuccessCount int                 `json:"success_count"`   // 成功数量
	FailedCount  int                 `json:"failed_count"`    // 失败数量
	ProcessTime  float64             `json:"process_time_ms"` // 总处理时间(毫秒)
}

// EmbeddingSimilarityRequest 相似度计算请求
type EmbeddingSimilarityRequest struct {
	Vector1 EmbeddingVector `json:"vector1"` // 第一个向量
	Vector2 EmbeddingVector `json:"vector2"` // 第二个向量
	Metric  string          `json:"metric"`  // 相似度度量方式 ("cosine", "euclidean", "manhattan")
}

// EmbeddingSimilarityResponse 相似度计算响应
type EmbeddingSimilarityResponse struct {
	Similarity float32 `json:"similarity"`         // 相似度值
	Metric     string  `json:"metric"`             // 使用的度量方式
	Distance   float32 `json:"distance,omitempty"` // 距离值（如果适用）
}

// EmbeddingSearchRequest 向量搜索请求
type EmbeddingSearchRequest struct {
	Query     EmbeddingVector   `json:"query"`               // 查询向量
	Database  []EmbeddingVector `json:"database"`            // 向量数据库
	TopK      int               `json:"top_k"`               // 返回TopK结果
	Metric    string            `json:"metric"`              // 相似度度量方式
	Threshold float32           `json:"threshold,omitempty"` // 相似度阈值
}

// EmbeddingSearchResult 搜索结果项
type EmbeddingSearchResult struct {
	Index    int             `json:"index"`              // 数据库中的索引
	Vector   EmbeddingVector `json:"vector"`             // 向量
	Score    float32         `json:"score"`              // 相似度分数
	Distance float32         `json:"distance,omitempty"` // 距离值
}

// EmbeddingSearchResponse 向量搜索响应
type EmbeddingSearchResponse struct {
	Results     []EmbeddingSearchResult `json:"results"`         // 搜索结果
	Total       int                     `json:"total"`           // 搜索总数
	ProcessTime float64                 `json:"process_time_ms"` // 处理时间(毫秒)
}

// JSON序列化方法

// MarshalJSON 实现JSON序列化
func (e EmbeddingVector) MarshalJSON() ([]byte, error) {
	return json.Marshal([]float32(e))
}

// UnmarshalJSON 实现JSON反序列化
func (e *EmbeddingVector) UnmarshalJSON(data []byte) error {
	var vec []float32
	if err := json.Unmarshal(data, &vec); err != nil {
		return err
	}
	*e = EmbeddingVector(vec)
	return nil
}

// String 返回向量的字符串表示
func (e EmbeddingVector) String() string {
	if e.IsEmpty() {
		return "[]"
	}
	if len(e) <= 10 {
		return fmt.Sprintf("%v", []float32(e))
	}
	return fmt.Sprintf("[%v...](dim=%d)", []float32(e[:5]), len(e))
}

// Equal 检查两个向量是否相等
func (e EmbeddingVector) Equal(other EmbeddingVector) bool {
	if len(e) != len(other) {
		return false
	}

	for i, v := range e {
		if v != other[i] {
			return false
		}
	}

	return true
}

// AlmostEqual 检查两个向量是否近似相等（考虑浮点误差）
func (e EmbeddingVector) AlmostEqual(other EmbeddingVector, epsilon float32) bool {
	if len(e) != len(other) {
		return false
	}

	for i, v := range e {
		if math.Abs(float64(v-other[i])) > float64(epsilon) {
			return false
		}
	}

	return true
}
