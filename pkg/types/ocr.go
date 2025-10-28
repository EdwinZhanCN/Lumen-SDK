package types

import (
	"fmt"
)

// OCRRequest OCR请求
type OCRRequest struct {
	Image     []byte                 `json:"image"`               // 图像数据
	MimeType  string                 `json:"mime_type"`           // 图像MIME类型
	ModelID   string                 `json:"model_id"`            // 模型ID
	Languages []string               `json:"languages,omitempty"` // 支持的语言列表
	Options   map[string]interface{} `json:"options,omitempty"`   // OCR选项
}

// OCRResponse OCR响应
type OCRResponse struct {
	TextBlocks  []*TextBlock           `json:"text_blocks"`          // 文本块列表
	FullText    string                 `json:"full_text"`            // 完整文本
	Confidence  float32                `json:"confidence"`           // 整体置信度
	ProcessTime float64                `json:"process_time_ms"`      // 处理时间(毫秒)
	ModelID     string                 `json:"model_id"`             // 使用的模型ID
	ImageSize   *ImageSize             `json:"image_size,omitempty"` // 图像尺寸
	Metadata    map[string]interface{} `json:"metadata,omitempty"`   // 响应元数据
}

// TextBlock 文本块
type TextBlock struct {
	BBox        *BoundingBox `json:"bbox"`                  // 边界框
	Text        string       `json:"text"`                  // 文本内容
	Confidence  float32      `json:"confidence"`            // 置信度
	Lines       []*TextLine  `json:"lines,omitempty"`       // 文本行
	Words       []*TextWord  `json:"words,omitempty"`       // 单词列表
	Language    string       `json:"language,omitempty"`    // 语言
	FontInfo    *FontInfo    `json:"font_info,omitempty"`   // 字体信息
	Orientation *Orientation `json:"orientation,omitempty"` // 文本方向
}

// TextLine 文本行
type TextLine struct {
	BBox       *BoundingBox `json:"bbox"`               // 边界框
	Text       string       `json:"text"`               // 文本内容
	Confidence float32      `json:"confidence"`         // 置信度
	Words      []*TextWord  `json:"words,omitempty"`    // 单词列表
	Baseline   *Baseline    `json:"baseline,omitempty"` // 基线信息
}

// TextWord 单词
type TextWord struct {
	BBox       *BoundingBox `json:"bbox"`                 // 边界框
	Text       string       `json:"text"`                 // 单词内容
	Confidence float32      `json:"confidence"`           // 置信度
	Characters []*Character `json:"characters,omitempty"` // 字符列表
}

// Character 字符
type Character struct {
	BBox       *BoundingBox `json:"bbox"`       // 边界框
	Text       string       `json:"text"`       // 字符内容
	Confidence float32      `json:"confidence"` // 置信度
}

// Baseline 基线信息
type Baseline struct {
	StartX float32 `json:"start_x"` // 起始X坐标
	StartY float32 `json:"start_y"` // 起始Y坐标
	EndX   float32 `json:"end_x"`   // 结束X坐标
	EndY   float32 `json:"end_y"`   // 结束Y坐标
}

// FontInfo 字体信息
type FontInfo struct {
	Family   string  `json:"family,omitempty"`    // 字体族
	Size     float32 `json:"size,omitempty"`      // 字体大小
	Weight   string  `json:"weight,omitempty"`    // 字体粗细
	Style    string  `json:"style,omitempty"`     // 字体样式
	IsBold   bool    `json:"is_bold,omitempty"`   // 是否粗体
	IsItalic bool    `json:"is_italic,omitempty"` // 是否斜体
}

// Orientation 文本方向
type Orientation struct {
	Angle     float32 `json:"angle"`          // 旋转角度
	Direction string  `json:"direction"`      // 方向 ("horizontal", "vertical")
	Flip      bool    `json:"flip,omitempty"` // 是否翻转
}

// NewTextBlock 创建新的文本块
func NewTextBlock(bbox *BoundingBox, text string, confidence float32) *TextBlock {
	return &TextBlock{
		BBox:       bbox,
		Text:       text,
		Confidence: confidence,
		Lines:      make([]*TextLine, 0),
		Words:      make([]*TextWord, 0),
	}
}

// AddLine 添加文本行
func (tb *TextBlock) AddLine(line *TextLine) {
	tb.Lines = append(tb.Lines, line)
}

// AddWord 添加单词
func (tb *TextBlock) AddWord(word *TextWord) {
	tb.Words = append(tb.Words, word)
}

// NewTextLine 创建新的文本行
func NewTextLine(bbox *BoundingBox, text string, confidence float32) *TextLine {
	return &TextLine{
		BBox:       bbox,
		Text:       text,
		Confidence: confidence,
		Words:      make([]*TextWord, 0),
	}
}

// NewTextWord 创建新的单词
func NewTextWord(bbox *BoundingBox, text string, confidence float32) *TextWord {
	return &TextWord{
		BBox:       bbox,
		Text:       text,
		Confidence: confidence,
		Characters: make([]*Character, 0),
	}
}

// NewCharacter 创建新的字符
func NewCharacter(bbox *BoundingBox, text string, confidence float32) *Character {
	return &Character{
		BBox:       bbox,
		Text:       text,
		Confidence: confidence,
	}
}

// OCRFeatures OCR特征提取请求
type OCRFeaturesRequest struct {
	OCRRequest
	ExtractFeatures bool     `json:"extract_features"` // 是否提取特征
	FeatureTypes    []string `json:"feature_types"`    // 特征类型列表
}

// OCRFeaturesResponse OCR特征提取响应
type OCRFeaturesResponse struct {
	*OCRResponse
	Features map[string]interface{} `json:"features,omitempty"` // 提取的特征
}

// TableDetection 表格检测请求
type TableDetectionRequest struct {
	OCRRequest
	DetectTables   bool `json:"detect_tables"`   // 是否检测表格
	DetectCells    bool `json:"detect_cells"`    // 是否检测单元格
	PreserveLayout bool `json:"preserve_layout"` // 是否保持布局
}

// TableDetectionResponse 表格检测响应
type TableDetectionResponse struct {
	*OCRResponse
	Tables []*Table `json:"tables,omitempty"` // 检测到的表格
}

// Table 表格
type Table struct {
	BBox       *BoundingBox `json:"bbox"`              // 表格边界框
	Rows       []*TableRow  `json:"rows"`              // 表格行
	Headers    []*TableRow  `json:"headers,omitempty"` // 表头
	Confidence float32      `json:"confidence"`        // 表格置信度
}

// TableRow 表格行
type TableRow struct {
	BBox      *BoundingBox `json:"bbox"`       // 行边界框
	Cells     []*TableCell `json:"cells"`      // 单元格列表
	IsHeader  bool         `json:"is_header"`  // 是否为表头
	RowNumber int          `json:"row_number"` // 行号
}

// TableCell 表格单元格
type TableCell struct {
	BBox       *BoundingBox `json:"bbox"`       // 单元格边界框
	Text       string       `json:"text"`       // 单元格文本
	Confidence float32      `json:"confidence"` // 置信度
	ColSpan    int          `json:"col_span"`   // 列跨度
	RowSpan    int          `json:"row_span"`   // 行跨度
	ColNumber  int          `json:"col_number"` // 列号
}

// DocumentAnalysis 文档分析请求
type DocumentAnalysisRequest struct {
	OCRRequest
	AnalyzeLayout bool `json:"analyze_layout"` // 是否分析布局
	ExtractFields bool `json:"extract_fields"` // 是否提取字段
	ClassifyType  bool `json:"classify_type"`  // 是否分类文档类型
}

// DocumentAnalysisResponse 文档分析响应
type DocumentAnalysisResponse struct {
	*OCRResponse
	DocumentType string                 `json:"document_type,omitempty"` // 文档类型
	Layout       *DocumentLayout        `json:"layout,omitempty"`        // 布局信息
	Fields       map[string]*Field      `json:"fields,omitempty"`        // 提取的字段
	Metadata     map[string]interface{} `json:"metadata,omitempty"`      // 文档元数据
}

// DocumentLayout 文档布局
type DocumentLayout struct {
	Sections []*LayoutSection `json:"sections,omitempty"` // 文档区块
	Title    string           `json:"title,omitempty"`    // 文档标题
	Margins  *Margins         `json:"margins,omitempty"`  // 页边距
}

// LayoutSection 布局区块
type LayoutSection struct {
	BBox       *BoundingBox     `json:"bbox"`               // 区块边界框
	Type       string           `json:"type"`               // 区块类型
	Content    string           `json:"content,omitempty"`  // 区块内容
	Confidence float32          `json:"confidence"`         // 置信度
	Children   []*LayoutSection `json:"children,omitempty"` // 子区块
}

// Margins 页边距
type Margins struct {
	Top    float32 `json:"top"`    // 上边距
	Bottom float32 `json:"bottom"` // 下边距
	Left   float32 `json:"left"`   // 左边距
	Right  float32 `json:"right"`  // 右边距
}

// Field 字段
type Field struct {
	Key        string       `json:"key"`                  // 字段键
	Value      string       `json:"value"`                // 字段值
	BBox       *BoundingBox `json:"bbox,omitempty"`       // 字段位置
	Confidence float32      `json:"confidence"`           // 置信度
	FieldType  string       `json:"field_type,omitempty"` // 字段类型
	Validated  bool         `json:"validated,omitempty"`  // 是否已验证
}

// HandwritingRecognition 手写识别请求
type HandwritingRecognitionRequest struct {
	OCRRequest
	LanguageModel string `json:"language_model,omitempty"` // 语言模型
	StrokeData    []byte `json:"stroke_data,omitempty"`    // 笔画数据（如果是手写输入）
}

// HandwritingRecognitionResponse 手写识别响应
type HandwritingRecognitionResponse struct {
	*OCRResponse
	Strokes      []*Stroke `json:"strokes,omitempty"`      // 识别的笔画
	Alternatives []string  `json:"alternatives,omitempty"` // 备选识别结果
}

// Stroke 笔画
type Stroke struct {
	Points     []*Point `json:"points"`     // 笔画点列表
	Confidence float32  `json:"confidence"` // 笔画置信度
}

// Point 点
type Point struct {
	X float32 `json:"x"`           // X坐标
	Y float32 `json:"y"`           // Y坐标
	T float32 `json:"t,omitempty"` // 时间戳（可选）
	P float32 `json:"p,omitempty"` // 压力（可选）
}

// BatchOCRRequest 批量OCR请求
type BatchOCRRequest struct {
	Requests       []OCRRequest `json:"requests"`        // OCR请求列表
	MaxConcurrency int          `json:"max_concurrency"` // 最大并发数
	BatchSize      int          `json:"batch_size"`      // 批处理大小
}

// BatchOCRResponse 批量OCR响应
type BatchOCRResponse struct {
	Results      []*OCRResponse `json:"results"`         // OCR结果列表
	TotalCount   int            `json:"total_count"`     // 总数量
	SuccessCount int            `json:"success_count"`   // 成功数量
	FailedCount  int            `json:"failed_count"`    // 失败数量
	ProcessTime  float64        `json:"process_time_ms"` // 总处理时间(毫秒)
}

// 辅助函数

// GetFullText 从文本块获取完整文本
func GetFullText(blocks []*TextBlock) string {
	if len(blocks) == 0 {
		return ""
	}

	var fullText string
	for i, block := range blocks {
		if i > 0 {
			fullText += "\n"
		}
		fullText += block.Text
	}

	return fullText
}

// GetBlocksByType 按类型获取文本块
func GetBlocksByType(blocks []*TextBlock, blockType string) []*TextBlock {
	var filtered []*TextBlock
	for _, block := range blocks {
		// 这里可以根据实际的类型判断逻辑进行过滤
		// 例如，通过字体信息、位置等来判断文本块类型
		filtered = append(filtered, block)
	}
	return filtered
}

// GetAverageConfidence 获取平均置信度
func GetAverageConfidence(blocks []*TextBlock) float32 {
	if len(blocks) == 0 {
		return 0.0
	}

	var sum float32
	for _, block := range blocks {
		sum += block.Confidence
	}

	return sum / float32(len(blocks))
}

// FilterBlocksByConfidence 按置信度过滤文本块
func FilterBlocksByConfidence(blocks []*TextBlock, threshold float32) []*TextBlock {
	var filtered []*TextBlock
	for _, block := range blocks {
		if block.Confidence >= threshold {
			filtered = append(filtered, block)
		}
	}
	return filtered
}

// MergeBlocks 合并相邻的文本块
func MergeBlocks(blocks []*TextBlock, maxDistance float32) []*TextBlock {
	if len(blocks) <= 1 {
		return blocks
	}

	// 简化实现：按Y坐标排序后合并相邻的块
	sorted := make([]*TextBlock, len(blocks))
	copy(sorted, blocks)

	// 这里应该实现更复杂的排序和合并逻辑
	// 为了简化，直接返回原始列表
	return sorted
}

// GetTextInArea 获取指定区域内的文本
func GetTextInArea(blocks []*TextBlock, area *BoundingBox) []string {
	var texts []string

	for _, block := range blocks {
		if block.BBox != nil && isInArea(block.BBox, area) {
			texts = append(texts, block.Text)
		}
	}

	return texts
}

// isInArea 检查边界框是否在指定区域内
func isInArea(bbox, area *BoundingBox) bool {
	if bbox == nil || area == nil {
		return false
	}

	return bbox.XMin >= area.XMin &&
		bbox.YMin >= area.YMin &&
		bbox.XMax <= area.XMax &&
		bbox.YMax <= area.YMax
}

// String 返回文本块的字符串表示
func (tb *TextBlock) String() string {
	if tb == nil {
		return "nil"
	}
	return fmt.Sprintf("TextBlock(text=\"%s\", confidence=%.2f)", tb.Text, tb.Confidence)
}

// String 返回文本行的字符串表示
func (tl *TextLine) String() string {
	if tl == nil {
		return "nil"
	}
	return fmt.Sprintf("TextLine(text=\"%s\", confidence=%.2f)", tl.Text, tl.Confidence)
}

// String 返回单词的字符串表示
func (tw *TextWord) String() string {
	if tw == nil {
		return "nil"
	}
	return fmt.Sprintf("TextWord(text=\"%s\", confidence=%.2f)", tw.Text, tw.Confidence)
}
