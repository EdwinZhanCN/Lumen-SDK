# Types Module

## 概述

Types模块是Lumen SDK的数据类型定义核心，提供了统一、类型安全的数据模型用于AI任务。该模块定义了各种AI任务的数据结构、请求响应格式以及相关的辅助类型，确保整个SDK内部数据交换的一致性和可维护性。

## 主要功能

### 🎯 任务类型定义
- **目标检测 (Detection)**: 物体检测、人脸检测等视觉任务
- **光学字符识别 (OCR)**: 文本识别、文档分析等OCR任务  
- **向量嵌入 (Embedding)**: 文本向量化和语义搜索
- **文本转语音 (TTS)**: 语音合成和音频生成

### 📊 核心数据结构
- 统一的请求/响应格式
- 标准化的错误处理
- 完整的元数据支持
- JSON序列化兼容

### 🛠️ 辅助工具
- 数据验证和转换
- 几何计算函数
- 向量相似度计算
- 文本处理工具

## 模块结构

```
pkg/types/
├── detection.go       # 目标检测相关类型
├── embedding.go       # 向量嵌入相关类型
├── ocr.go            # OCR识别相关类型
├── tts.go            # 文本转语音相关类型
└── README.md         # 本文档
```

## 核心类型详解

### 1. 目标检测 (Detection)

#### BoundingBox - 边界框
定义物体在图像中的位置和大小。

```go
type BoundingBox struct {
    XMin float32 `json:"xmin"` // 左上角X坐标
    YMin float32 `json:"ymin"` // 左上角Y坐标
    XMax float32 `json:"xmax"` // 右下角X坐标
    YMax float32 `json:"ymax"` // 右下角Y坐标
}
```

**核心方法:**
```go
// 创建边界框
box := NewBoundingBox(10, 10, 100, 100)

// 计算几何属性
width := box.Width()        // 宽度
height := box.Height()      // 高度
area := box.Area()          // 面积
x, y := box.Center()        // 中心点

// 几何操作
expanded := box.Expand(1.2)     // 扩大20%
clipped := box.Clip(512, 512)   // 裁剪到指定尺寸
iou := box.IoU(otherBox)        // 计算IoU

// 验证
isValid := box.IsValid()        // 检查是否有效
```

#### DetectionResult - 检测结果
单个物体的检测结果。

```go
type DetectionResult struct {
    Box        *BoundingBox `json:"box"`         // 边界框
    ClassID    int          `json:"class_id"`    // 类别ID
    ClassName  string       `json:"class_name"`  // 类别名称
    Confidence float32      `json:"confidence"`  // 置信度
}
```

### 2. 向量嵌入 (Embedding)

#### EmbeddingVector - 嵌入向量
用于表示文本或图像的向量表示。

```go
type EmbeddingVector []float32
```

**核心方法:**
```go
// 创建向量
vec := NewEmbeddingVector([]float32{0.1, 0.2, 0.3, 0.4})
vec64 := NewEmbeddingVectorFromFloat64([]float64{0.1, 0.2, 0.3, 0.4})

// 向量属性
dim := vec.Dim()              // 向量维度
norm := vec.Magnitude()       // 向量模长

// 向量操作
normalized := vec.Normalize() // 归一化

// 相似度计算
cosine, _ := vec.CosineSimilarity(otherVec)
euclidean := vec.EuclideanDistance(otherVec)
dotProduct := vec.DotProduct(otherVec)
```

### 3. OCR识别 (OCR)

#### TextBlock - 文本块
OCR识别出的文本单元。

```go
type TextBlock struct {
    BBox       *BoundingBox `json:"bbox"`        // 边界框
    Text       string       `json:"text"`        // 文本内容
    Confidence float32      `json:"confidence"` // 置信度
    Language   string       `json:"language"`   // 语言代码
}
```

#### OCRRequest/Response - OCR请求响应
```go
type OCRRequest struct {
    Image     []byte                 `json:"image"`               // 图像数据
    MimeType  string                 `json:"mime_type"`           // 图像MIME类型
    ModelID   string                 `json:"model_id"`            // 模型ID
    Languages []string               `json:"languages,omitempty"` // 支持的语言列表
    Options   map[string]interface{} `json:"options,omitempty"`   // OCR选项
}

type OCRResponse struct {
    TextBlocks  []*TextBlock           `json:"text_blocks"`          // 文本块列表
    FullText    string                 `json:"full_text"`            // 完整文本
    Confidence  float32                `json:"confidence"`           // 整体置信度
    ProcessTime float64                `json:"process_time_ms"`      // 处理时间(毫秒)
    ModelID     string                 `json:"model_id"`             // 使用的模型ID
    ImageSize   *ImageSize             `json:"image_size,omitempty"` // 图像尺寸
    Metadata    map[string]interface{} `json:"metadata,omitempty"`   // 响应元数据
}
```

**辅助函数:**
```go
// 文本提取
fullText := GetFullText(textBlocks)
avgConf := GetAverageConfidence(textBlocks)

// 文本过滤
filtered := FilterTextBlocksByConfidence(textBlocks, 0.8)
byLanguage := FilterTextBlocksByLanguage(textBlocks, "zh-CN")
```

### 4. 文本转语音 (TTS)

#### TTSRequest/Response - TTS请求响应
```go
type TTSRequest struct {
    Text         string                 `json:"text"`                  // 要转换的文本
    VoiceID      string                 `json:"voice_id"`              // 语音ID
    ModelID      string                 `json:"model_id"`              // 模型ID
    Language     string                 `json:"language,omitempty"`    // 语言代码
    Speed        float32                `json:"speed,omitempty"`       // 语速 (0.5-2.0)
    Pitch        float32                `json:"pitch,omitempty"`       // 音调 (-20.0 to 20.0)
    Volume       float32                `json:"volume,omitempty"`      // 音量 (0.0-1.0)
    OutputFormat string                 `json:"output_format"`         // 输出格式 ("wav", "mp3", "ogg")
    SampleRate   int                    `json:"sample_rate,omitempty"` // 采样率
    SSML         string                 `json:"ssml,omitempty"`        // SSML文本（优先于text）
    Options      map[string]interface{} `json:"options,omitempty"`     // TTS选项
}

type TTSResponse struct {
    AudioData   []byte                 `json:"audio_data"`         // 音频数据
    Format      string                 `json:"format"`             // 音频格式
    SampleRate  int                    `json:"sample_rate"`        // 采样率
    Duration    float64                `json:"duration"`           // 音频时长(秒)
    Size        int                    `json:"size"`               // 数据大小(字节)
    ModelID     string                 `json:"model_id"`           // 使用的模型ID
    VoiceID     string                 `json:"voice_id"`           // 使用的语音ID
    ProcessTime float64                `json:"process_time_ms"`    // 处理时间(毫秒)
    Metadata    map[string]interface{} `json:"metadata,omitempty"` // 响应元数据
}
```

**辅助函数:**
```go
// 音频时长估算
duration := EstimateAudioDuration("Hello world", 1.0)

// 请求验证
if err := ValidateTTSRequest(req); err != nil {
    return fmt.Errorf("invalid TTS request: %w", err)
}

// 音频格式转换
if err := ConvertAudioFormat(audio, "wav", "mp3"); err != nil {
    return fmt.Errorf("format conversion failed: %w", err)
}
```

## 使用指南

### 目标检测示例

```go
// 创建检测请求
detectionReq := &DetectionRequest{
    Image:        imageData,
    MimeType:     "image/jpeg",
    ModelID:      "yolo-v5",
    Threshold:    0.5,
    MaxDetections: 100,
}

// 处理检测结果
detections := []*DetectionResult{
    {
        Box:        NewBoundingBox(100, 100, 200, 200),
        ClassID:    1,
        ClassName:  "person",
        Confidence: 0.85,
    },
    {
        Box:        NewBoundingBox(300, 150, 450, 300),
        ClassID:    2,
        ClassName:  "car",
        Confidence: 0.92,
    },
}

// 非极大值抑制 (NMS)
filtered := NMS(detections, 0.5)

// 按置信度过滤
highConf := FilterDetectionsByConfidence(detections, 0.8)

// 按类别过滤
persons := FilterDetectionsByClass(detections, "person")
cars := FilterDetectionsByClass(detections, "car")
```

### 向量嵌入示例

```go
// 创建文本嵌入向量
queryVec := NewEmbeddingVector([]float32{0.1, 0.2, 0.3, 0.4, 0.5})
docVecs := []EmbeddingVector{
    NewEmbeddingVector([]float32{0.2, 0.3, 0.4, 0.5, 0.6}),
    NewEmbeddingVector([]float32{0.8, 0.7, 0.6, 0.5, 0.4}),
}

// 向量搜索
results := VectorSearch(queryVec, docVecs, "cosine", 3)

// 相似度计算
for i, docVec := range docVecs {
    similarity, _ := queryVec.CosineSimilarity(docVec)
    fmt.Printf("Document %d: similarity=%.3f\n", i, similarity)
}

// 向量聚合
averageVec := AverageVectors(docVecs)
```

### OCR识别示例

```go
// 创建OCR请求
ocrReq := &OCRRequest{
    Image:     imageData,
    MimeType:  "image/png",
    ModelID:   "tesseract",
    Languages: []string{"zh-CN", "en"},
    Options: map[string]interface{}{
        "preprocess": true,
        "enhance":    true,
    },
}

// 处理OCR结果
ocrResp := &OCRResponse{
    TextBlocks: []*TextBlock{
        {
            BBox:       NewBoundingBox(10, 10, 200, 50),
            Text:       "Hello World",
            Confidence: 0.95,
            Language:   "en",
        },
        {
            BBox:       NewBoundingBox(10, 60, 150, 100),
            Text:       "你好世界",
            Confidence: 0.88,
            Language:   "zh-CN",
        },
    },
    FullText:   "Hello World\n你好世界",
    Confidence: 0.91,
}

// 提取和过滤文本
fullText := GetFullText(ocrResp.TextBlocks)
avgConf := GetAverageConfidence(ocrResp.TextBlocks)
highConfBlocks := FilterTextBlocksByConfidence(ocrResp.TextBlocks, 0.9)
```

### TTS合成示例

```go
// 创建TTS请求
ttsReq := &TTSRequest{
    Text:         "Hello, this is a test of text-to-speech synthesis.",
    VoiceID:      "voice-001",
    ModelID:      "tts-model-1",
    Language:     "en-US",
    Speed:        1.0,
    Pitch:        0.0,
    Volume:       0.8,
    OutputFormat: "wav",
    SampleRate:   22050,
}

// 验证请求
if err := ValidateTTSRequest(ttsReq); err != nil {
    return fmt.Errorf("invalid TTS request: %w", err)
}

// 处理TTS响应
ttsResp := &TTSResponse{
    AudioData:   audioData,
    Format:      "wav",
    SampleRate:  22050,
    Duration:    3.5,
    Size:        len(audioData),
    ModelID:     "tts-model-1",
    VoiceID:     "voice-001",
    ProcessTime: 1200,
}

// 音频质量检查
if ttsResp.Duration < 0.1 {
    return fmt.Errorf("audio duration too short: %.2fs", ttsResp.Duration)
}
```

## 高级功能

### 1. 数据验证

所有类型都提供验证功能：

```go
// 边界框验证
box := NewBoundingBox(10, 10, 100, 100)
if !box.IsValid() {
    return fmt.Errorf("invalid bounding box")
}

// 向量验证
vec := NewEmbeddingVector([]float32{0.1, 0.2})
if vec.Dim() == 0 {
    return fmt.Errorf("empty vector")
}

// TTS请求验证
if err := ValidateTTSRequest(req); err != nil {
    return fmt.Errorf("validation failed: %w", err)
}
```

### 2. 批量处理

```go
// 批量检测
batchDetectionReq := &BatchDetectionRequest{
    Images:       [][]byte{img1, img2, img3},
    MimeType:     "image/jpeg",
    ModelID:      "yolo-v5",
    MaxBatchSize: 4,
}

// 批量OCR
batchOCRReq := &BatchOCRRequest{
    Images:    [][]byte{doc1, doc2},
    Languages: []string{"en"},
    ModelID:   "tesseract",
}
```

### 3. 数据转换

```go
// 坐标系转换
normalizedBox := NormalizeBoundingBox(box, imageWidth, imageHeight)
pixelBox := DenormalizeBoundingBox(normalizedBox, imageWidth, imageHeight)

// 向量格式转换
jsonBytes, _ := json.Marshal(vec)
base64Str := base64.StdEncoding.EncodeToString(jsonBytes)
```

## 性能优化

### 1. 内存管理
- 使用对象池减少GC压力
- 避免不必要的数据拷贝
- 及时释放大型数据结构

### 2. 计算优化
- 向量化操作减少循环
- 缓存重复计算结果
- 使用SIMD指令优化

### 3. 并发处理
- 支持并发向量计算
- 无锁数据结构
- 批量处理优化

## 错误处理

### 错误类型定义

```go
// 验证错误
type ValidationError struct {
    Field   string
    Value   interface{}
    Message string
}

// 计算错误
type ComputationError struct {
    Operation string
    Reason    string
}
```

### 错误处理模式

```go
// 使用包装错误
if err := processImage(img); err != nil {
    return fmt.Errorf("image processing failed: %w", err)
}

// 错误类型检查
if errors.Is(err, ErrInvalidImage) {
    // 处理特定错误
}

// 错误链追踪
fmt.Printf("%+v\n", err) // 打印完整错误链
```

## 扩展指南

### 1. 添加新的任务类型

```go
// 定义新的请求类型
type CustomRequest struct {
    Input   interface{}            `json:"input"`
    Options map[string]interface{} `json:"options,omitempty"`
}

// 定义响应类型
type CustomResponse struct {
    Result   interface{}            `json:"result"`
    Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// 实现验证方法
func (r *CustomRequest) Validate() error {
    // 验证逻辑
    return nil
}
```

### 2. 添加辅助函数

```go
// 添加新的几何计算
func (b *BoundingBox) AspectRatio() float32 {
    return b.Width() / b.Height()
}

// 添加新的向量操作
func (e EmbeddingVector) Max() float32 {
    max := float32(0)
    for _, v := range e {
        if v > max {
            max = v
        }
    }
    return max
}
```

## 测试指南

### 单元测试示例

```go
func TestBoundingBox(t *testing.T) {
    box := NewBoundingBox(0, 0, 10, 10)
    
    // 测试基本属性
    assert.Equal(t, float32(10), box.Width())
    assert.Equal(t, float32(10), box.Height())
    assert.Equal(t, float32(100), box.Area())
    
    // 测试中心点
    x, y := box.Center()
    assert.Equal(t, float32(5), x)
    assert.Equal(t, float32(5), y)
    
    // 测试有效性
    assert.True(t, box.IsValid())
    
    // 测试IoU计算
    other := NewBoundingBox(5, 5, 15, 15)
    iou := box.IoU(other)
    assert.InDelta(t, 0.1428, iou, 0.001)
}
```

### 基准测试

```go
func BenchmarkCosineSimilarity(b *testing.B) {
    v1 := NewEmbeddingVector(make([]float32, 1000))
    v2 := NewEmbeddingVector(make([]float32, 1000))
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        v1.CosineSimilarity(v2)
    }
}
```

## 版本兼容性

### 向后兼容性
- 保持JSON标签不变
- 新增字段使用omitempty
- 不修改现有方法签名

### 版本迁移
- 提供迁移工具
- 详细的迁移文档
- 渐进式升级支持

## API参考

### BoundingBox方法
| 方法 | 描述 | 返回 |
|------|------|------|
| `Width()` | 计算宽度 | float32 |
| `Height()` | 计算高度 | float32 |
| `Area()` | 计算面积 | float32 |
| `Center()` | 计算中心点 | (float32, float32) |
| `Expand(factor)` | 按比例扩展 | *BoundingBox |
| `Clip(w, h)` | 裁剪到指定尺寸 | *BoundingBox |
| `IoU(other)` | 计算IoU | float32 |

### EmbeddingVector方法
| 方法 | 描述 | 返回 |
|------|------|------|
| `Dim()` | 向量维度 | int |
| `Magnitude()` | 向量模长 | float32 |
| `Normalize()` | 归一化 | EmbeddingVector |
| `CosineSimilarity(other)` | 余弦相似度 | (float64, error) |
| `EuclideanDistance(other)` | 欧几里得距离 | float32 |
| `DotProduct(other)` | 点积 | float32 |

## 贡献指南

### 代码规范
- 遵循Go官方代码规范
- 使用gofmt格式化代码
- 添加完整的文档注释
- 编写单元测试和基准测试

### 提交流程
1. Fork项目
2. 创建功能分支
3. 编写代码和测试
4. 提交Pull Request
5. 代码审查和合并

## 更新日志

### v1.0.0
- 初始版本发布
- 实现基础数据类型
- 提供完整的辅助函数
- 添加单元测试

### v1.1.0
- 添加批量处理支持
- 优化向量计算性能
- 增强数据验证功能
- 添加更多辅助工具

### v1.2.0 (计划中)
- 支持更多音频格式
- 添加高级图像处理
- 实现向量数据库接口
- 增强并发处理能力

## 许可证

本项目采用 Apache 2.0 许可证，详情请参见 `LICENSE` 文件。