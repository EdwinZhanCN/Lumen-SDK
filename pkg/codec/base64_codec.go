package codec

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
)

// Base64Codec Base64编解码器
type Base64Codec struct {
	encoding   *base64.Encoding
	lineLength int
}

// NewBase64Codec 创建新的Base64编解码器
func NewBase64Codec() *Base64Codec {
	return &Base64Codec{
		encoding:   base64.StdEncoding,
		lineLength: 76, // 标准Base64行长度
	}
}

// NewBase64CodecWithEncoding 创建使用指定编码的Base64编解码器
func NewBase64CodecWithEncoding(encoding *base64.Encoding) *Base64Codec {
	return &Base64Codec{
		encoding:   encoding,
		lineLength: 76,
	}
}

// NewBase64CodecWithURLSafe 创建URL安全的Base64编解码器
func NewBase64CodecWithURLSafe() *Base64Codec {
	return &Base64Codec{
		encoding:   base64.URLEncoding,
		lineLength: 0, // URL安全编码不分行
	}
}

// Name 返回编解码器名称
func (b *Base64Codec) Name() string {
	return "Base64Codec"
}

// MimeTypes 返回支持的MIME类型列表
func (b *Base64Codec) MimeTypes() []string {
	return []string{
		"text/plain",
		"application/base64",
		"text/base64",
		"application/octet-stream",
	}
}

// Encode 将数据编码为Base64字节数据
func (b *Base64Codec) Encode(data interface{}) ([]byte, error) {
	var input []byte

	switch v := data.(type) {
	case []byte:
		input = v
	case string:
		input = []byte(v)
	case []rune:
		input = []byte(string(v))
	case io.Reader:
		var buf bytes.Buffer
		if _, err := io.Copy(&buf, v); err != nil {
			return nil, NewCodecError("", b.Name(),
				fmt.Sprintf("failed to read from io.Reader: %v", err))
		}
		input = buf.Bytes()
	default:
		// 尝试转换为字符串
		str := fmt.Sprintf("%v", v)
		input = []byte(str)
	}

	if len(input) == 0 {
		return []byte(""), nil
	}

	// 计算所需的输出长度
	outputLen := b.encoding.EncodedLen(len(input))
	output := make([]byte, outputLen)

	// 编码数据
	b.encoding.Encode(output, input)

	// 如果需要分行
	if b.lineLength > 0 {
		output = b.addLineBreaks(output)
	}

	return output, nil
}

// EncodeToString 将数据编码为Base64字符串
func (b *Base64Codec) EncodeToString(data interface{}) (string, error) {
	encoded, err := b.Encode(data)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

// EncodeWithPadding 编码数据并指定是否添加填充
func (b *Base64Codec) EncodeWithPadding(data interface{}, padding bool) ([]byte, error) {
	if !padding {
		// 使用无填充的编码
		encoded := b.encodeWithoutPadding(data)
		return []byte(encoded), nil
	}

	return b.Encode(data)
}

// Decode 将Base64字节数据解码
func (b *Base64Codec) Decode(data []byte, target interface{}) error {
	if len(data) == 0 {
		return NewCodecError("", b.Name(), "data cannot be empty")
	}

	// 移除空白字符和换行符
	cleanData := b.cleanData(data)

	// 计算所需的输出长度
	outputLen := b.encoding.DecodedLen(len(cleanData))
	output := make([]byte, outputLen)

	// 解码数据
	n, err := b.encoding.Decode(output, cleanData)
	if err != nil {
		return NewCodecError("", b.Name(),
			fmt.Sprintf("failed to decode base64: %v", err))
	}

	// 截断到实际长度
	output = output[:n]

	// 根据target类型设置解码结果
	switch t := target.(type) {
	case *[]byte:
		*t = output
	case *string:
		*t = string(output)
	case **[]byte:
		*t = &output
	case *map[string]interface{}:
		if t == nil {
			return NewCodecError("", b.Name(), "target map pointer cannot be nil")
		}
		if *t == nil {
			*t = make(map[string]interface{})
		}
		(*t)["data"] = output
		(*t)["string"] = string(output)
		(*t)["length"] = len(output)
		(*t)["original_length"] = len(cleanData)
	default:
		return NewCodecError("", b.Name(),
			fmt.Sprintf("unsupported target type: %T", target))
	}

	return nil
}

// DecodeFromString 从Base64字符串解码
func (b *Base64Codec) DecodeFromString(encoded string, target interface{}) error {
	return b.Decode([]byte(encoded), target)
}

// DecodeWithPadding 解码数据，处理无填充的情况
func (b *Base64Codec) DecodeWithPadding(data []byte, target interface{}, padding bool) error {
	if !padding {
		// 添加必要的填充
		data = b.addPadding(data)
	}

	return b.Decode(data, target)
}

// Validate 验证数据是否为有效的Base64
func (b *Base64Codec) Validate(data interface{}) error {
	if data == nil {
		return NewCodecError("", b.Name(), "data cannot be nil")
	}

	var input []byte

	switch v := data.(type) {
	case []byte:
		input = v
	case string:
		input = []byte(v)
	default:
		input = []byte(fmt.Sprintf("%v", v))
	}

	if len(input) == 0 {
		return nil // 空字符串是有效的Base64
	}

	// 清理数据
	cleanData := b.cleanData(input)

	// 检查长度是否是4的倍数
	if len(cleanData)%4 != 0 {
		return NewCodecError("", b.Name(),
			"base64 data length must be a multiple of 4")
	}

	// 尝试解码以验证
	_, err := b.encoding.DecodeString(string(cleanData))
	if err != nil {
		return NewCodecError("", b.Name(),
			fmt.Sprintf("invalid base64 data: %v", err))
	}

	return nil
}

// IsValidBase64 检查数据是否为有效的Base64
func (b *Base64Codec) IsValidBase64(data interface{}) bool {
	return b.Validate(data) == nil
}

// EstimateDecodedSize 估算解码后的数据大小
func (b *Base64Codec) EstimateDecodedSize(encodedLength int) int {
	// Base64编码会使数据增长约33%
	return encodedLength * 3 / 4
}

// EstimateEncodedSize 估算编码后的数据大小
func (b *Base64Codec) EstimateEncodedSize(decodedLength int) int {
	encodedLen := b.encoding.EncodedLen(decodedLength)
	if b.lineLength > 0 {
		// 加上换行符的大小
		lines := (encodedLen + b.lineLength - 1) / b.lineLength
		return encodedLen + lines - 1
	}
	return encodedLen
}

// SplitLines 将Base64数据分割成行
func (b *Base64Codec) SplitLines(encoded string) []string {
	if b.lineLength <= 0 {
		return []string{encoded}
	}

	var lines []string
	for i := 0; i < len(encoded); i += b.lineLength {
		end := i + b.lineLength
		if end > len(encoded) {
			end = len(encoded)
		}
		lines = append(lines, encoded[i:end])
	}

	return lines
}

// JoinLines 将多行Base64数据合并
func (b *Base64Codec) JoinLines(lines []string) string {
	return strings.Join(lines, "")
}

// 辅助方法

// cleanData 清理Base64数据，移除空白字符和换行符
func (b *Base64Codec) cleanData(data []byte) []byte {
	var cleaned []byte
	for _, b := range data {
		if b != ' ' && b != '\t' && b != '\n' && b != '\r' {
			cleaned = append(cleaned, b)
		}
	}
	return cleaned
}

// addLineBreaks 添加换行符
func (b *Base64Codec) addLineBreaks(data []byte) []byte {
	if b.lineLength <= 0 {
		return data
	}

	var result []byte
	for i := 0; i < len(data); i += b.lineLength {
		end := i + b.lineLength
		if end > len(data) {
			end = len(data)
		}
		result = append(result, data[i:end]...)
		if end < len(data) {
			result = append(result, '\n')
		}
	}

	return result
}

// encodeWithoutPadding 编码数据但不添加填充
func (b *Base64Codec) encodeWithoutPadding(data interface{}) string {
	input, err := b.toBytes(data)
	if err != nil {
		return ""
	}

	if len(input) == 0 {
		return ""
	}

	// 编码数据
	encoded := b.encoding.EncodeToString(input)

	// 移除填充字符 '='
	encoded = strings.TrimRight(encoded, "=")

	return encoded
}

// addPadding 添加必要的填充
func (b *Base64Codec) addPadding(data []byte) []byte {
	cleanData := b.cleanData(data)

	// 计算需要添加的填充字符数
	padding := (4 - len(cleanData)%4) % 4

	result := make([]byte, len(cleanData)+padding)
	copy(result, cleanData)

	for i := len(cleanData); i < len(result); i++ {
		result[i] = '='
	}

	return result
}

// toBytes 将各种数据类型转换为字节数组
func (b *Base64Codec) toBytes(data interface{}) ([]byte, error) {
	switch v := data.(type) {
	case []byte:
		return v, nil
	case string:
		return []byte(v), nil
	case []rune:
		return []byte(string(v)), nil
	case io.Reader:
		var buf bytes.Buffer
		if _, err := io.Copy(&buf, v); err != nil {
			return nil, fmt.Errorf("failed to read from io.Reader: %v", err)
		}
		return buf.Bytes(), nil
	default:
		str := fmt.Sprintf("%v", v)
		return []byte(str), nil
	}
}

// SetEncoding 设置Base64编码
func (b *Base64Codec) SetEncoding(encoding *base64.Encoding) {
	b.encoding = encoding
}

// SetLineLength 设置行长度
func (b *Base64Codec) SetLineLength(length int) {
	b.lineLength = length
}

// GetEncoding 获取Base64编码
func (b *Base64Codec) GetEncoding() *base64.Encoding {
	return b.encoding
}

// GetLineLength 获取行长度
func (b *Base64Codec) GetLineLength() int {
	return b.lineLength
}

// IsURLSafe 检查是否为URL安全编码
func (b *Base64Codec) IsURLSafe() bool {
	return b.encoding == base64.URLEncoding
}

// String 返回编解码器的字符串表示
func (b *Base64Codec) String() string {
	var encodingType string
	switch b.encoding {
	case base64.StdEncoding:
		encodingType = "standard"
	case base64.URLEncoding:
		encodingType = "url-safe"
	case base64.RawStdEncoding:
		encodingType = "raw-standard"
	case base64.RawURLEncoding:
		encodingType = "raw-url-safe"
	default:
		encodingType = "custom"
	}

	return fmt.Sprintf("Base64Codec{name=%s, encoding=%s, line_length=%d, mime_types=%v}",
		b.Name(), encodingType, b.lineLength, b.MimeTypes())
}
