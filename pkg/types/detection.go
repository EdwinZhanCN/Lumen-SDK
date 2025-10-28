package types

import (
	"fmt"
)

// BoundingBox 边界框
type BoundingBox struct {
	XMin float32 `json:"xmin"` // 左上角X坐标
	YMin float32 `json:"ymin"` // 左上角Y坐标
	XMax float32 `json:"xmax"` // 右下角X坐标
	YMax float32 `json:"ymax"` // 右下角Y坐标
}

// NewBoundingBox 创建新的边界框
func NewBoundingBox(xmin, ymin, xmax, ymax float32) *BoundingBox {
	return &BoundingBox{
		XMin: xmin,
		YMin: ymin,
		XMax: xmax,
		YMax: ymax,
	}
}

// Width 返回边界框宽度
func (b *BoundingBox) Width() float32 {
	return b.XMax - b.XMin
}

// Height 返回边界框高度
func (b *BoundingBox) Height() float32 {
	return b.YMax - b.YMin
}

// Area 返回边界框面积
func (b *BoundingBox) Area() float32 {
	return b.Width() * b.Height()
}

// Center 返回边界框中心点
func (b *BoundingBox) Center() (x, y float32) {
	x = (b.XMin + b.XMax) / 2.0
	y = (b.YMin + b.YMax) / 2.0
	return
}

// IsValid 检查边界框是否有效
func (b *BoundingBox) IsValid() bool {
	return b.XMin < b.XMax && b.YMin < b.YMax &&
		b.XMin >= 0 && b.YMin >= 0 &&
		b.XMax >= 0 && b.YMax >= 0
}

// Expand 按比例扩展边界框
func (b *BoundingBox) Expand(ratio float32) *BoundingBox {
	width := b.Width()
	height := b.Height()

	horizontalExpand := width * (ratio - 1.0) / 2.0
	verticalExpand := height * (ratio - 1.0) / 2.0

	return &BoundingBox{
		XMin: b.XMin - horizontalExpand,
		YMin: b.YMin - verticalExpand,
		XMax: b.XMax + horizontalExpand,
		YMax: b.YMax + verticalExpand,
	}
}

// Clip 裁剪到指定范围
func (b *BoundingBox) Clip(xmax, ymax float32) *BoundingBox {
	return &BoundingBox{
		XMin: max(0, b.XMin),
		YMin: max(0, b.YMin),
		XMax: min(xmax, b.XMax),
		YMax: min(ymax, b.YMax),
	}
}

// Intersection 计算两个边界框的交集
func (b *BoundingBox) Intersection(other *BoundingBox) *BoundingBox {
	if !b.IsValid() || !other.IsValid() {
		return nil
	}

	xmin := max(b.XMin, other.XMin)
	ymin := max(b.YMin, other.YMin)
	xmax := min(b.XMax, other.XMax)
	ymax := min(b.YMax, other.YMax)

	if xmin >= xmax || ymin >= ymax {
		return nil
	}

	return NewBoundingBox(xmin, ymin, xmax, ymax)
}

// Union 计算两个边界框的并集
func (b *BoundingBox) Union(other *BoundingBox) *BoundingBox {
	if !b.IsValid() {
		return other.Clone()
	}
	if !other.IsValid() {
		return b.Clone()
	}

	return NewBoundingBox(
		min(b.XMin, other.XMin),
		min(b.YMin, other.YMin),
		max(b.XMax, other.XMax),
		max(b.YMax, other.YMax),
	)
}

// IoU 计算交并比 (Intersection over Union)
func (b *BoundingBox) IoU(other *BoundingBox) float32 {
	intersection := b.Intersection(other)
	if intersection == nil {
		return 0.0
	}

	unionArea := b.Area() + other.Area() - intersection.Area()
	if unionArea == 0.0 {
		return 0.0
	}

	return intersection.Area() / unionArea
}

// Clone 克隆边界框
func (b *BoundingBox) Clone() *BoundingBox {
	if b == nil {
		return nil
	}
	return &BoundingBox{
		XMin: b.XMin,
		YMin: b.YMin,
		XMax: b.XMax,
		YMax: b.YMax,
	}
}

// String 返回边界框的字符串表示
func (b *BoundingBox) String() string {
	if b == nil {
		return "nil"
	}
	return fmt.Sprintf("Box(%.2f,%.2f,%.2f,%.2f)", b.XMin, b.YMin, b.XMax, b.YMax)
}

// Detection 检测结果
type Detection struct {
	Box        *BoundingBox           `json:"box"`                // 边界框
	ClassID    int                    `json:"class_id"`           // 类别ID
	ClassName  string                 `json:"class_name"`         // 类别名称
	Confidence float32                `json:"confidence"`         // 置信度
	Label      string                 `json:"label,omitempty"`    // 标签
	Metadata   map[string]interface{} `json:"metadata,omitempty"` // 元数据
}

// NewDetection 创建新的检测结果
func NewDetection(box *BoundingBox, classID int, className string, confidence float32) *Detection {
	return &Detection{
		Box:        box,
		ClassID:    classID,
		ClassName:  className,
		Confidence: confidence,
		Metadata:   make(map[string]interface{}),
	}
}

// IsValid 检查检测结果是否有效
func (d *Detection) IsValid() bool {
	return d.Box != nil && d.Box.IsValid() && d.ClassID >= 0 && d.Confidence >= 0.0 && d.Confidence <= 1.0
}

// WithLabel 添加标签
func (d *Detection) WithLabel(label string) *Detection {
	d.Label = label
	return d
}

// WithMetadata 添加元数据
func (d *Detection) WithMetadata(key string, value interface{}) *Detection {
	if d.Metadata == nil {
		d.Metadata = make(map[string]interface{})
	}
	d.Metadata[key] = value
	return d
}

// Clone 克隆检测结果
func (d *Detection) Clone() *Detection {
	if d == nil {
		return nil
	}

	clone := &Detection{
		Box:        d.Box.Clone(),
		ClassID:    d.ClassID,
		ClassName:  d.ClassName,
		Confidence: d.Confidence,
		Label:      d.Label,
		Metadata:   make(map[string]interface{}),
	}

	for k, v := range d.Metadata {
		clone.Metadata[k] = v
	}

	return clone
}

// DetectionRequest 检测请求
type DetectionRequest struct {
	Image         []byte                 `json:"image"`             // 图像数据
	MimeType      string                 `json:"mime_type"`         // 图像MIME类型
	ModelID       string                 `json:"model_id"`          // 模型ID
	Threshold     float32                `json:"threshold"`         // 置信度阈值
	MaxDetections int                    `json:"max_detections"`    // 最大检测数量
	Classes       []string               `json:"classes,omitempty"` // 指定检测的类别
	Options       map[string]interface{} `json:"options,omitempty"` // 检测选项
}

// DetectionResponse 检测响应
type DetectionResponse struct {
	Detections  []*Detection           `json:"detections"`           // 检测结果列表
	Count       int                    `json:"count"`                // 检测数量
	ProcessTime float64                `json:"process_time_ms"`      // 处理时间(毫秒)
	ModelID     string                 `json:"model_id"`             // 使用的模型ID
	ImageSize   *ImageSize             `json:"image_size,omitempty"` // 图像尺寸
	Metadata    map[string]interface{} `json:"metadata,omitempty"`   // 响应元数据
}

// ImageSize 图像尺寸
type ImageSize struct {
	Width    int `json:"width"`    // 图像宽度
	Height   int `json:"height"`   // 图像高度
	Channels int `json:"channels"` // 通道数
}

// NewImageSize 创建图像尺寸
func NewImageSize(width, height, channels int) *ImageSize {
	return &ImageSize{
		Width:    width,
		Height:   height,
		Channels: channels,
	}
}

// Area 返回图像面积
func (i *ImageSize) Area() int {
	return i.Width * i.Height
}

// AspectRatio 返回宽高比
func (i *ImageSize) AspectRatio() float32 {
	if i.Height == 0 {
		return 0.0
	}
	return float32(i.Width) / float32(i.Height)
}

// FaceDetection 人脸检测结果
type FaceDetection struct {
	*Detection
	Landmarks  []*Landmark     `json:"landmarks,omitempty"`  // 人脸关键点
	Embedding  EmbeddingVector `json:"embedding,omitempty"`  // 人脸嵌入向量
	Attributes *FaceAttributes `json:"attributes,omitempty"` // 人脸属性
}

// Landmark 关键点
type Landmark struct {
	X          float32 `json:"x"`                    // X坐标
	Y          float32 `json:"y"`                    // Y坐标
	Name       string  `json:"name"`                 // 关键点名称
	Visibility float32 `json:"visibility,omitempty"` // 可见性
}

// NewLandmark 创建关键点
func NewLandmark(x, y float32, name string) *Landmark {
	return &Landmark{
		X:          x,
		Y:          y,
		Name:       name,
		Visibility: 1.0,
	}
}

// FaceAttributes 人脸属性
type FaceAttributes struct {
	Age      *AgeRange `json:"age,omitempty"`       // 年龄范围
	Gender   *Gender   `json:"gender,omitempty"`    // 性别
	Emotion  *Emotion  `json:"emotion,omitempty"`   // 情绪
	SkinTone *SkinTone `json:"skin_tone,omitempty"` // 肤色
	Glasses  *Glasses  `json:"glasses,omitempty"`   // 眼镜
	Mask     *Mask     `json:"mask,omitempty"`      // 口罩
}

// AgeRange 年龄范围
type AgeRange struct {
	Min int `json:"min"` // 最小年龄
	Max int `json:"max"` // 最大年龄
}

// Gender 性别
type Gender struct {
	Type       string  `json:"type"`       // 性别类型
	Confidence float32 `json:"confidence"` // 置信度
}

// Emotion 情绪
type Emotion struct {
	Type       string  `json:"type"`       // 情绪类型
	Confidence float32 `json:"confidence"` // 置信度
}

// SkinTone 肤色
type SkinTone struct {
	Type       string  `json:"type"`       // 肤色类型
	Confidence float32 `json:"confidence"` // 置信度
}

// Glasses 眼镜
type Glasses struct {
	Wearing    bool    `json:"wearing"`    // 是否佩戴眼镜
	Type       string  `json:"type"`       // 眼镜类型
	Confidence float32 `json:"confidence"` // 置信度
}

// Mask 口罩
type Mask struct {
	Wearing    bool    `json:"wearing"`    // 是否佩戴口罩
	Type       string  `json:"type"`       // 口罩类型
	Confidence float32 `json:"confidence"` // 置信度
}

// ObjectDetectionRequest 物体检测请求
type ObjectDetectionRequest struct {
	DetectionRequest
	TargetClasses []string `json:"target_classes,omitempty"` // 目标类别
	TrackObjects  bool     `json:"track_objects,omitempty"`  // 是否跟踪物体
}

// FaceDetectionRequest 人脸检测请求
type FaceDetectionRequest struct {
	DetectionRequest
	DetectLandmarks   bool `json:"detect_landmarks,omitempty"`   // 是否检测关键点
	ExtractEmbedding  bool `json:"extract_embedding,omitempty"`  // 是否提取嵌入向量
	AnalyzeAttributes bool `json:"analyze_attributes,omitempty"` // 是否分析属性
}

// BatchDetectionRequest 批量检测请求
type BatchDetectionRequest struct {
	Requests       []DetectionRequest `json:"requests"`        // 检测请求列表
	MaxConcurrency int                `json:"max_concurrency"` // 最大并发数
	BatchSize      int                `json:"batch_size"`      // 批处理大小
}

// BatchDetectionResponse 批量检测响应
type BatchDetectionResponse struct {
	Results      []*DetectionResponse `json:"results"`         // 检测结果列表
	TotalCount   int                  `json:"total_count"`     // 总数量
	SuccessCount int                  `json:"success_count"`   // 成功数量
	FailedCount  int                  `json:"failed_count"`    // 失败数量
	ProcessTime  float64              `json:"process_time_ms"` // 总处理时间(毫秒)
}

// 辅助函数
func min(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func max(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

// NMS 非极大值抑制
func NMS(detections []*Detection, iouThreshold float32) []*Detection {
	if len(detections) == 0 {
		return []*Detection{}
	}

	// 按置信度降序排序
	sorted := make([]*Detection, len(detections))
	copy(sorted, detections)

	// 简单的冒泡排序，实际应用中应该使用更高效的排序算法
	for i := 0; i < len(sorted)-1; i++ {
		for j := 0; j < len(sorted)-i-1; j++ {
			if sorted[j].Confidence < sorted[j+1].Confidence {
				sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
			}
		}
	}

	var result []*Detection
	suppressed := make([]bool, len(sorted))

	for i := 0; i < len(sorted); i++ {
		if suppressed[i] {
			continue
		}

		result = append(result, sorted[i])

		for j := i + 1; j < len(sorted); j++ {
			if !suppressed[j] &&
				sorted[i].Box.IoU(sorted[j].Box) > iouThreshold &&
				sorted[i].ClassID == sorted[j].ClassID {
				suppressed[j] = true
			}
		}
	}

	return result
}

// FilterDetectionsByClass 按类别过滤检测结果
func FilterDetectionsByClass(detections []*Detection, classNames ...string) []*Detection {
	if len(classNames) == 0 {
		return detections
	}

	classMap := make(map[string]bool)
	for _, name := range classNames {
		classMap[name] = true
	}

	var filtered []*Detection
	for _, detection := range detections {
		if classMap[detection.ClassName] {
			filtered = append(filtered, detection)
		}
	}

	return filtered
}

// FilterDetectionsByConfidence 按置信度过滤检测结果
func FilterDetectionsByConfidence(detections []*Detection, threshold float32) []*Detection {
	var filtered []*Detection
	for _, detection := range detections {
		if detection.Confidence >= threshold {
			filtered = append(filtered, detection)
		}
	}
	return filtered
}
