package types

import (
	"fmt"
)

// TTSRequest 文本转语音请求
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

// TTSResponse 文本转语音响应
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

// VoiceInfo 语音信息
type VoiceInfo struct {
	ID          string            `json:"id"`                    // 语音ID
	Name        string            `json:"name"`                  // 语音名称
	Language    string            `json:"language"`              // 语言
	Gender      string            `json:"gender"`                // 性别 ("male", "female", "neutral")
	Age         string            `json:"age,omitempty"`         // 年龄段
	Accent      string            `json:"accent,omitempty"`      // 口音
	Description string            `json:"description"`           // 描述
	PreviewURL  string            `json:"preview_url,omitempty"` // 预览音频URL
	SampleRate  int               `json:"sample_rate"`           // 支持的采样率
	Formats     []string          `json:"formats"`               // 支持的格式
	Features    []string          `json:"features"`              // 支持的特性
	Tags        map[string]string `json:"tags,omitempty"`        // 标签
}

// TTSCapability TTS能力信息
type TTSCapability struct {
	ModelID          string       `json:"model_id"`          // 模型ID
	ModelName        string       `json:"model_name"`        // 模型名称
	Voices           []*VoiceInfo `json:"voices"`            // 支持的语音列表
	Languages        []string     `json:"languages"`         // 支持的语言
	Formats          []string     `json:"formats"`           // 支持的输出格式
	SampleRates      []int        `json:"sample_rates"`      // 支持的采样率
	MaxTextLength    int          `json:"max_text_length"`   // 最大文本长度
	SpeedRange       *Range       `json:"speed_range"`       // 语速范围
	PitchRange       *Range       `json:"pitch_range"`       // 音调范围
	VolumeRange      *Range       `json:"volume_range"`      // 音量范围
	SupportsSSML     bool         `json:"supports_ssml"`     // 是否支持SSML
	SupportsEmphasis bool         `json:"supports_emphasis"` // 是否支持强调
	SupportsProsody  bool         `json:"supports_prosody"`  // 是否支持韵律
	Features         []string     `json:"features"`          // 特性列表
}

// Range 数值范围
type Range struct {
	Min float32 `json:"min"` // 最小值
	Max float32 `json:"max"` // 最大值
}

// NewRange 创建新的范围
func NewRange(min, max float32) *Range {
	return &Range{
		Min: min,
		Max: max,
	}
}

// Contains 检查值是否在范围内
func (r *Range) Contains(value float32) bool {
	return r != nil && value >= r.Min && value <= r.Max
}

// Clamp 将值限制在范围内
func (r *Range) Clamp(value float32) float32 {
	if r == nil {
		return value
	}
	if value < r.Min {
		return r.Min
	}
	if value > r.Max {
		return r.Max
	}
	return value
}

// BatchTTSRequest 批量TTS请求
type BatchTTSRequest struct {
	Requests       []TTSRequest `json:"requests"`        // TTS请求列表
	MaxConcurrency int          `json:"max_concurrency"` // 最大并发数
	BatchSize      int          `json:"batch_size"`      // 批处理大小
	MergeOutput    bool         `json:"merge_output"`    // 是否合并输出
}

// BatchTTSResponse 批量TTS响应
type BatchTTSResponse struct {
	Results      []*TTSResponse `json:"results"`                // TTS结果列表
	TotalCount   int            `json:"total_count"`            // 总数量
	SuccessCount int            `json:"success_count"`          // 成功数量
	FailedCount  int            `json:"failed_count"`           // 失败数量
	ProcessTime  float64        `json:"process_time_ms"`        // 总处理时间(毫秒)
	MergedAudio  []byte         `json:"merged_audio,omitempty"` // 合并后的音频数据
}

// VoiceCustomizationRequest 语音定制请求
type VoiceCustomizationRequest struct {
	BaseVoiceID  string                 `json:"base_voice_id"` // 基础语音ID
	Samples      []byte                 `json:"samples"`       // 语音样本数据
	SampleFormat string                 `json:"sample_format"` // 样本格式
	Description  string                 `json:"description"`   // 描述
	Features     map[string]interface{} `json:"features"`      // 定制特性
}

// VoiceCustomizationResponse 语音定制响应
type VoiceCustomizationResponse struct {
	VoiceID      string                 `json:"voice_id"`                // 新语音ID
	Status       string                 `json:"status"`                  // 定制状态
	Progress     float32                `json:"progress"`                // 进度 (0.0-1.0)
	EstimatedETA int                    `json:"estimated_eta_seconds"`   // 预计完成时间(秒)
	ErrorMessage string                 `json:"error_message,omitempty"` // 错误信息
	Metadata     map[string]interface{} `json:"metadata"`                // 元数据
}

// SpeechSynthesisMark 语音合成标记
type SpeechSynthesisMark struct {
	Name  string  `json:"name"`            // 标记名称
	Time  float64 `json:"time"`            // 标记时间(秒)
	Type  string  `json:"type"`            // 标记类型
	Value string  `json:"value,omitempty"` // 标记值
}

// TTSStreamingRequest 流式TTS请求
type TTSStreamingRequest struct {
	TTSRequest
	ChunkSize    int  `json:"chunk_size,omitempty"` // 块大小
	RealTime     bool `json:"real_time"`            // 是否实时
	IncludeMarks bool `json:"include_marks"`        // 是否包含标记
}

// TTSStreamingResponse 流式TTS响应
type TTSStreamingResponse struct {
	ChunkIndex  int                    `json:"chunk_index"`     // 块索引
	AudioChunk  []byte                 `json:"audio_chunk"`     // 音频块
	IsFinal     bool                   `json:"is_final"`        // 是否为最后一块
	Marks       []*SpeechSynthesisMark `json:"marks,omitempty"` // 标记列表
	Progress    float32                `json:"progress"`        // 进度 (0.0-1.0)
	ProcessTime float64                `json:"process_time_ms"` // 处理时间(毫秒)
}

// AudioFormat 音频格式信息
type AudioFormat struct {
	Name        string `json:"name"`              // 格式名称
	Extension   string `json:"extension"`         // 文件扩展名
	MimeType    string `json:"mime_type"`         // MIME类型
	Codec       string `json:"codec"`             // 编解码器
	Bitrate     int    `json:"bitrate,omitempty"` // 比特率
	Channels    int    `json:"channels"`          // 声道数
	SampleRate  int    `json:"sample_rate"`       // 采样率
	Compression bool   `json:"compression"`       // 是否压缩
}

// 常用音频格式
var (
	FormatWAV = &AudioFormat{
		Name:        "WAV",
		Extension:   ".wav",
		MimeType:    "audio/wav",
		Codec:       "pcm",
		Channels:    1,
		SampleRate:  22050,
		Compression: false,
	}

	FormatMP3 = &AudioFormat{
		Name:        "MP3",
		Extension:   ".mp3",
		MimeType:    "audio/mpeg",
		Codec:       "mp3",
		Bitrate:     128000,
		Channels:    2,
		SampleRate:  44100,
		Compression: true,
	}

	FormatOGG = &AudioFormat{
		Name:        "OGG",
		Extension:   ".ogg",
		MimeType:    "audio/ogg",
		Codec:       "vorbis",
		Bitrate:     96000,
		Channels:    2,
		SampleRate:  44100,
		Compression: true,
	}
)

// GetAudioFormat 根据格式名称获取音频格式
func GetAudioFormat(format string) *AudioFormat {
	switch format {
	case "wav":
		return FormatWAV
	case "mp3":
		return FormatMP3
	case "ogg":
		return FormatOGG
	default:
		return FormatWAV // 默认返回WAV
	}
}

// ValidateTTSRequest 验证TTS请求
func ValidateTTSRequest(req *TTSRequest) error {
	if req.Text == "" && req.SSML == "" {
		return fmt.Errorf("text or ssml must be provided")
	}

	if req.VoiceID == "" {
		return fmt.Errorf("voice_id must be provided")
	}

	if req.ModelID == "" {
		return fmt.Errorf("model_id must be provided")
	}

	// 验证输出格式
	format := GetAudioFormat(req.OutputFormat)
	if format == nil {
		return fmt.Errorf("unsupported output format: %s", req.OutputFormat)
	}

	// 验证参数范围
	if req.Speed < 0.1 || req.Speed > 3.0 {
		return fmt.Errorf("speed must be between 0.1 and 3.0")
	}

	if req.Pitch < -20.0 || req.Pitch > 20.0 {
		return fmt.Errorf("pitch must be between -20.0 and 20.0")
	}

	if req.Volume < 0.0 || req.Volume > 1.0 {
		return fmt.Errorf("volume must be between 0.0 and 1.0")
	}

	return nil
}

// EstimateAudioDuration 估算音频时长
func EstimateAudioDuration(text string, speed float32) float64 {
	// 简化估算：平均每个字符0.1秒，考虑语速
	charCount := len([]rune(text))
	baseDuration := float64(charCount) * 0.1 // 基础时长
	return baseDuration / float64(speed)
}

// String 返回语音信息的字符串表示
func (v *VoiceInfo) String() string {
	if v == nil {
		return "nil"
	}
	return fmt.Sprintf("VoiceInfo(id=%s, name=%s, lang=%s, gender=%s)",
		v.ID, v.Name, v.Language, v.Gender)
}

// String 返回TTS能力的字符串表示
func (c *TTSCapability) String() string {
	if c == nil {
		return "nil"
	}
	return fmt.Sprintf("TTSCapability(model=%s, voices=%d, languages=%d)",
		c.ModelID, len(c.Voices), len(c.Languages))
}

// GetVoiceByID 根据ID获取语音信息
func (c *TTSCapability) GetVoiceByID(voiceID string) *VoiceInfo {
	for _, voice := range c.Voices {
		if voice.ID == voiceID {
			return voice
		}
	}
	return nil
}

// GetVoicesByLanguage 根据语言获取语音列表
func (c *TTSCapability) GetVoicesByLanguage(language string) []*VoiceInfo {
	var voices []*VoiceInfo
	for _, voice := range c.Voices {
		if voice.Language == language {
			voices = append(voices, voice)
		}
	}
	return voices
}

// GetVoicesByGender 根据性别获取语音列表
func (c *TTSCapability) GetVoicesByGender(gender string) []*VoiceInfo {
	var voices []*VoiceInfo
	for _, voice := range c.Voices {
		if voice.Gender == gender {
			voices = append(voices, voice)
		}
	}
	return voices
}

// SupportsFormat 检查是否支持指定格式
func (c *TTSCapability) SupportsFormat(format string) bool {
	for _, f := range c.Formats {
		if f == format {
			return true
		}
	}
	return false
}

// SupportsSampleRate 检查是否支持指定采样率
func (c *TTSCapability) SupportsSampleRate(sampleRate int) bool {
	for _, rate := range c.SampleRates {
		if rate == sampleRate {
			return true
		}
	}
	return false
}

// SupportsLanguage 检查是否支持指定语言
func (c *TTSCapability) SupportsLanguage(language string) bool {
	for _, lang := range c.Languages {
		if lang == language {
			return true
		}
	}
	return false
}
