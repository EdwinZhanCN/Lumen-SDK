package codec

import (
	"fmt"
	"sync"

	"Lumen-SDK/pkg/utils"
)

// Codec 编解码器接口
type Codec interface {
	// MimeTypes 返回支持的MIME类型列表
	MimeTypes() []string

	// Encode 将Go类型编码为字节数据
	Encode(data interface{}) ([]byte, error)

	// Decode 将字节数据解码为Go类型
	Decode(data []byte, target interface{}) error

	// Validate 验证数据是否符合编解码器要求
	Validate(data interface{}) error

	// Name 返回编解码器名称
	Name() string
}

// CodecRegistry 编解码器注册表
type CodecRegistry struct {
	codecs map[string]Codec // 按MIME类型索引
	mu     sync.RWMutex
}

// NewCodecRegistry 创建新的编解码器注册表
func NewCodecRegistry() *CodecRegistry {
	return &CodecRegistry{
		codecs: make(map[string]Codec),
	}
}

// Register 注册编解码器
func (cr *CodecRegistry) Register(mimeTypes []string, codec Codec) error {
	if len(mimeTypes) == 0 {
		return utils.InvalidError("mime types cannot be empty")
	}

	if codec == nil {
		return utils.InvalidError("codec cannot be nil")
	}

	cr.mu.Lock()
	defer cr.mu.Unlock()

	// 验证编解码器
	for _, mimeType := range mimeTypes {
		if mimeType == "" {
			return utils.InvalidError("mime type cannot be empty")
		}

		// 检查是否已存在
		if existing, exists := cr.codecs[mimeType]; exists {
			return utils.InvalidError(
				fmt.Sprintf("mime type %s already registered with codec %s",
					mimeType, existing.Name()))
		}

		// 验证编解码器是否支持该MIME类型
		supportedTypes := codec.MimeTypes()
		found := false
		for _, supportedType := range supportedTypes {
			if supportedType == mimeType {
				found = true
				break
			}
		}

		if !found {
			return utils.InvalidError(
				fmt.Sprintf("codec %s does not support mime type %s",
					codec.Name(), mimeType))
		}

		cr.codecs[mimeType] = codec
	}

	return nil
}

// Unregister 注销编解码器
func (cr *CodecRegistry) Unregister(mimeType string) bool {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	if _, exists := cr.codecs[mimeType]; exists {
		delete(cr.codecs, mimeType)
		return true
	}

	return false
}

// Get 获取指定MIME类型的编解码器
func (cr *CodecRegistry) Get(mimeType string) (Codec, error) {
	cr.mu.RLock()
	defer cr.mu.RUnlock()

	codec, exists := cr.codecs[mimeType]
	if !exists {
		return nil, utils.NotFoundError(
			fmt.Sprintf("no codec found for mime type: %s", mimeType))
	}

	return codec, nil
}

// GetOrDefault 获取编解码器，如果不存在则返回默认编解码器
func (cr *CodecRegistry) GetOrDefault(mimeType string, defaultCodec Codec) Codec {
	codec, err := cr.Get(mimeType)
	if err != nil {
		return defaultCodec
	}
	return codec
}

// Exists 检查指定MIME类型的编解码器是否存在
func (cr *CodecRegistry) Exists(mimeType string) bool {
	cr.mu.RLock()
	defer cr.mu.RUnlock()

	_, exists := cr.codecs[mimeType]
	return exists
}

// List 列出所有已注册的MIME类型
func (cr *CodecRegistry) List() []string {
	cr.mu.RLock()
	defer cr.mu.RUnlock()

	mimeTypes := make([]string, 0, len(cr.codecs))
	for mimeType := range cr.codecs {
		mimeTypes = append(mimeTypes, mimeType)
	}

	return mimeTypes
}

// ListCodecs 列出所有编解码器及其支持的MIME类型
func (cr *CodecRegistry) ListCodecs() map[string][]string {
	cr.mu.RLock()
	defer cr.mu.RUnlock()

	result := make(map[string][]string)

	for mimeType, codec := range cr.codecs {
		codecName := codec.Name()
		if _, exists := result[codecName]; !exists {
			result[codecName] = []string{}
		}
		result[codecName] = append(result[codecName], mimeType)
	}

	return result
}

// Encode 编码数据
func (cr *CodecRegistry) Encode(mimeType string, data interface{}) ([]byte, error) {
	codec, err := cr.Get(mimeType)
	if err != nil {
		return nil, err
	}

	// 验证数据
	if err := codec.Validate(data); err != nil {
		return nil, utils.Wrap(err, utils.ErrCodeInvalid,
			fmt.Sprintf("data validation failed for mime type %s", mimeType))
	}

	// 编码数据
	encoded, err := codec.Encode(data)
	if err != nil {
		return nil, utils.Wrap(err, utils.ErrCodeInternal,
			fmt.Sprintf("encoding failed for mime type %s", mimeType))
	}

	return encoded, nil
}

// Decode 解码数据
func (cr *CodecRegistry) Decode(mimeType string, data []byte, target interface{}) error {
	codec, err := cr.Get(mimeType)
	if err != nil {
		return err
	}

	// 解码数据
	if err := codec.Decode(data, target); err != nil {
		return utils.Wrap(err, utils.ErrCodeInternal,
			fmt.Sprintf("decoding failed for mime type %s", mimeType))
	}

	return nil
}

// SafeEncode 安全编码数据，包含错误恢复
func (cr *CodecRegistry) SafeEncode(mimeType string, data interface{}) ([]byte, error) {
	var result []byte
	err := utils.SafeExecute(func() error {
		var encodeErr error
		result, encodeErr = cr.Encode(mimeType, data)
		return encodeErr
	})
	return result, err
}

// SafeDecode 安全解码数据，包含错误恢复
func (cr *CodecRegistry) SafeDecode(mimeType string, data []byte, target interface{}) error {
	return utils.SafeExecute(func() error {
		return cr.Decode(mimeType, data, target)
	})
}

// Validate 验证数据是否符合指定MIME类型的要求
func (cr *CodecRegistry) Validate(mimeType string, data interface{}) error {
	codec, err := cr.Get(mimeType)
	if err != nil {
		return err
	}

	return codec.Validate(data)
}

// GetCodecByName 根据编解码器名称获取编解码器
func (cr *CodecRegistry) GetCodecByName(name string) (Codec, error) {
	cr.mu.RLock()
	defer cr.mu.RUnlock()

	for _, codec := range cr.codecs {
		if codec.Name() == name {
			return codec, nil
		}
	}

	return nil, utils.NotFoundError(fmt.Sprintf("no codec found with name: %s", name))
}

// GetStats 获取注册表统计信息
func (cr *CodecRegistry) GetStats() map[string]interface{} {
	cr.mu.RLock()
	defer cr.mu.RUnlock()

	stats := map[string]interface{}{
		"total_mime_types": len(cr.codecs),
		"total_codecs":     cr.countUniqueCodecs(),
		"mime_types":       cr.List(),
		"codec_details":    cr.ListCodecs(),
	}

	return stats
}

// countUniqueCodecs 计算唯一编解码器数量
func (cr *CodecRegistry) countUniqueCodecs() int {
	seen := make(map[string]bool)
	for _, codec := range cr.codecs {
		seen[codec.Name()] = true
	}
	return len(seen)
}

// Clear 清空注册表
func (cr *CodecRegistry) Clear() {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	cr.codecs = make(map[string]Codec)
}

// Size 返回注册表大小
func (cr *CodecRegistry) Size() int {
	cr.mu.RLock()
	defer cr.mu.RUnlock()

	return len(cr.codecs)
}

// IsEmpty 检查注册表是否为空
func (cr *CodecRegistry) IsEmpty() bool {
	return cr.Size() == 0
}

// 全局默认注册表
var defaultRegistry *CodecRegistry
var once sync.Once

// GetDefaultRegistry 获取默认编解码器注册表
func GetDefaultRegistry() *CodecRegistry {
	once.Do(func() {
		defaultRegistry = NewCodecRegistry()
		// 注册默认编解码器
		registerDefaultCodecs(defaultRegistry)
	})
	return defaultRegistry
}

// RegisterCodec 在默认注册表中注册编解码器
func RegisterCodec(mimeTypes []string, codec Codec) error {
	return GetDefaultRegistry().Register(mimeTypes, codec)
}

// GetCodec 从默认注册表获取编解码器
func GetCodec(mimeType string) (Codec, error) {
	return GetDefaultRegistry().Get(mimeType)
}

// EncodeData 使用默认注册表编码数据
func EncodeData(mimeType string, data interface{}) ([]byte, error) {
	return GetDefaultRegistry().Encode(mimeType, data)
}

// DecodeData 使用默认注册表解码数据
func DecodeData(mimeType string, data []byte, target interface{}) error {
	return GetDefaultRegistry().Decode(mimeType, data, target)
}

// ValidateData 使用默认注册表验证数据
func ValidateData(mimeType string, data interface{}) error {
	return GetDefaultRegistry().Validate(mimeType, data)
}

// registerDefaultCodecs 注册默认编解码器
func registerDefaultCodecs(registry *CodecRegistry) {
	// 注册JSON编解码器
	jsonCodec := NewJSONCodec()
	registry.Register([]string{
		"application/json",
		"text/json",
		"application/json; charset=utf-8",
	}, jsonCodec)

	// 注册图像编解码器
	imageCodec := NewImageCodec()
	registry.Register([]string{
		"image/jpeg",
		"image/jpg",
		"image/png",
		"image/gif",
	}, imageCodec)

	// 注册Base64编解码器
	base64Codec := NewBase64Codec()
	registry.Register([]string{
		"text/plain",
		"application/base64",
		"text/base64",
	}, base64Codec)
}

// CodecError 编解码器错误类型
type CodecError struct {
	MimeType  string `json:"mime_type"`
	CodecName string `json:"codec_name"`
	Message   string `json:"message"`
}

func (e *CodecError) Error() string {
	return fmt.Sprintf("codec error for %s (%s): %s", e.MimeType, e.CodecName, e.Message)
}

// NewCodecError 创建编解码器错误
func NewCodecError(mimeType, codecName, message string) *CodecError {
	return &CodecError{
		MimeType:  mimeType,
		CodecName: codecName,
		Message:   message,
	}
}

// 支持的MIME类型常量
const (
	MimeJSON           = "application/json"
	MimeText           = "text/plain"
	MimeJPEG           = "image/jpeg"
	MimeJPG            = "image/jpg"
	MimePNG            = "image/png"
	MimeGIF            = "image/gif"
	MimeOctetStream    = "application/octet-stream"
	MimeFormURLEncoded = "application/x-www-form-urlencoded"
	MimeFormData       = "multipart/form-data"
)

// IsSupportedType 检查是否为支持的MIME类型
func IsSupportedType(mimeType string) bool {
	registry := GetDefaultRegistry()
	return registry.Exists(mimeType)
}

// GetSupportedTypes 获取所有支持的MIME类型
func GetSupportedTypes() []string {
	registry := GetDefaultRegistry()
	return registry.List()
}

// DetectType 尝试检测数据的MIME类型
func DetectType(data []byte) string {
	// 简单的MIME类型检测
	if len(data) == 0 {
		return MimeOctetStream
	}

	// 检测JSON
	if isJSON(data) {
		return MimeJSON
	}

	// 检测图像
	if imageType := detectImageType(data); imageType != "" {
		return imageType
	}

	// 检测文本
	if isText(data) {
		return MimeText
	}

	return MimeOctetStream
}

// isJSON 检测是否为JSON数据
func isJSON(data []byte) bool {
	if len(data) == 0 {
		return false
	}

	// 跳过空白字符
	i := 0
	for i < len(data) && (data[i] == ' ' || data[i] == '\t' || data[i] == '\n' || data[i] == '\r') {
		i++
	}

	if i >= len(data) {
		return false
	}

	// JSON必须以{或[开始
	return data[i] == '{' || data[i] == '['
}

// detectImageType 检测图像类型
func detectImageType(data []byte) string {
	if len(data) < 8 {
		return ""
	}

	// JPEG: FF D8 FF
	if data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return MimeJPEG
	}

	// PNG: 89 50 4E 47 0D 0A 1A 0A
	if len(data) >= 8 &&
		data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 &&
		data[4] == 0x0D && data[5] == 0x0A && data[6] == 0x1A && data[7] == 0x0A {
		return MimePNG
	}

	// GIF: 47 49 46 38
	if len(data) >= 4 &&
		data[0] == 0x47 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x38 {
		return MimeGIF
	}

	return ""
}

// isText 检测是否为文本数据
func isText(data []byte) bool {
	if len(data) == 0 {
		return false
	}

	// 简单的文本检测：检查是否包含可打印字符
	textBytes := 0
	for _, b := range data {
		if (b >= 32 && b <= 126) || b == '\t' || b == '\n' || b == '\r' {
			textBytes++
		}
	}

	// 如果超过90%是可打印字符，认为是文本
	return float64(textBytes)/float64(len(data)) > 0.9
}
