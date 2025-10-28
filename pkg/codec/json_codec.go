package codec

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
)

// JSONCodec JSON编解码器
type JSONCodec struct {
	encoder *json.Encoder
	decoder *json.Decoder
}

// NewJSONCodec 创建新的JSON编解码器
func NewJSONCodec() *JSONCodec {
	return &JSONCodec{}
}

// Name 返回编解码器名称
func (j *JSONCodec) Name() string {
	return "JSONCodec"
}

// MimeTypes 返回支持的MIME类型列表
func (j *JSONCodec) MimeTypes() []string {
	return []string{
		"application/json",
		"text/json",
		"application/json; charset=utf-8",
	}
}

// Encode 将Go类型编码为JSON字节数据
func (j *JSONCodec) Encode(data interface{}) ([]byte, error) {
	if data == nil {
		return []byte("null"), nil
	}

	// 使用json.Marshal进行编码
	result, err := json.Marshal(data)
	if err != nil {
		return nil, NewCodecError("", j.Name(),
			fmt.Sprintf("failed to encode data: %v", err))
	}

	return result, nil
}

// EncodeWithIndent 使用缩进编码JSON
func (j *JSONCodec) EncodeWithIndent(data interface{}, prefix, indent string) ([]byte, error) {
	if data == nil {
		return []byte("null"), nil
	}

	result, err := json.MarshalIndent(data, prefix, indent)
	if err != nil {
		return nil, NewCodecError("", j.Name(),
			fmt.Sprintf("failed to encode data with indent: %v", err))
	}

	return result, nil
}

// EncodeToWriter 将数据编码到io.Writer
func (j *JSONCodec) EncodeToWriter(w io.Writer, data interface{}) error {
	if data == nil {
		_, err := w.Write([]byte("null"))
		return err
	}

	// 每次都创建新的encoder，因为json.Encoder不支持Reset
	j.encoder = json.NewEncoder(w)

	if err := j.encoder.Encode(data); err != nil {
		return NewCodecError("", j.Name(),
			fmt.Sprintf("failed to encode to writer: %v", err))
	}

	return nil
}

// Decode 将JSON字节数据解码为Go类型
func (j *JSONCodec) Decode(data []byte, target interface{}) error {
	if target == nil {
		return NewCodecError("", j.Name(), "target cannot be nil")
	}

	if len(data) == 0 {
		return NewCodecError("", j.Name(), "data cannot be empty")
	}

	// 检查target是否为指针
	targetValue := reflect.ValueOf(target)
	if targetValue.Kind() != reflect.Ptr {
		return NewCodecError("", j.Name(), "target must be a pointer")
	}

	if err := json.Unmarshal(data, target); err != nil {
		return NewCodecError("", j.Name(),
			fmt.Sprintf("failed to decode data: %v", err))
	}

	return nil
}

// DecodeFromReader 从io.Reader解码JSON数据
func (j *JSONCodec) DecodeFromReader(r io.Reader, target interface{}) error {
	if target == nil {
		return NewCodecError("", j.Name(), "target cannot be nil")
	}

	// 每次都创建新的decoder，因为json.Decoder不支持Reset
	j.decoder = json.NewDecoder(r)

	if err := j.decoder.Decode(target); err != nil {
		return NewCodecError("", j.Name(),
			fmt.Sprintf("failed to decode from reader: %v", err))
	}

	return nil
}

// Validate 验证数据是否可以被JSON编码
func (j *JSONCodec) Validate(data interface{}) error {
	if data == nil {
		return nil // null是有效的JSON值
	}

	// 尝试编码数据以验证
	_, err := j.Encode(data)
	if err != nil {
		return NewCodecError("", j.Name(),
			fmt.Sprintf("data validation failed: %v", err))
	}

	return nil
}

// ValidateJSON 验证JSON字符串是否有效
func (j *JSONCodec) ValidateJSON(jsonStr string) error {
	if jsonStr == "" {
		return NewCodecError("", j.Name(), "json string cannot be empty")
	}

	var temp interface{}
	if err := json.Unmarshal([]byte(jsonStr), &temp); err != nil {
		return NewCodecError("", j.Name(),
			fmt.Sprintf("invalid json format: %v", err))
	}

	return nil
}

// IsValidJSON 检查字符串是否为有效JSON
func (j *JSONCodec) IsValidJSON(jsonStr string) bool {
	return j.ValidateJSON(jsonStr) == nil
}

// PrettyPrint 格式化JSON字符串
func (j *JSONCodec) PrettyPrint(jsonStr string) (string, error) {
	if jsonStr == "" {
		return "", NewCodecError("", j.Name(), "json string cannot be empty")
	}

	var temp interface{}
	if err := json.Unmarshal([]byte(jsonStr), &temp); err != nil {
		return "", NewCodecError("", j.Name(),
			fmt.Sprintf("failed to parse json: %v", err))
	}

	pretty, err := json.MarshalIndent(temp, "", "  ")
	if err != nil {
		return "", NewCodecError("", j.Name(),
			fmt.Sprintf("failed to format json: %v", err))
	}

	return string(pretty), nil
}

// Compact 压缩JSON字符串
func (j *JSONCodec) Compact(jsonStr string) (string, error) {
	if jsonStr == "" {
		return "", NewCodecError("", j.Name(), "json string cannot be empty")
	}

	var temp interface{}
	if err := json.Unmarshal([]byte(jsonStr), &temp); err != nil {
		return "", NewCodecError("", j.Name(),
			fmt.Sprintf("failed to parse json: %v", err))
	}

	compact, err := json.Marshal(temp)
	if err != nil {
		return "", NewCodecError("", j.Name(),
			fmt.Sprintf("failed to compact json: %v", err))
	}

	return string(compact), nil
}

// GetJSONType 获取JSON值的类型
func (j *JSONCodec) GetJSONType(jsonStr string) (string, error) {
	if jsonStr == "" {
		return "", NewCodecError("", j.Name(), "json string cannot be empty")
	}

	var temp interface{}
	if err := json.Unmarshal([]byte(jsonStr), &temp); err != nil {
		return "", NewCodecError("", j.Name(),
			fmt.Sprintf("failed to parse json: %v", err))
	}

	switch temp.(type) {
	case nil:
		return "null", nil
	case bool:
		return "boolean", nil
	case float64:
		return "number", nil
	case string:
		return "string", nil
	case []interface{}:
		return "array", nil
	case map[string]interface{}:
		return "object", nil
	default:
		return "unknown", nil
	}
}

// ExtractField 从JSON中提取字段值
func (j *JSONCodec) ExtractField(jsonStr, fieldPath string) (interface{}, error) {
	if jsonStr == "" {
		return nil, NewCodecError("", j.Name(), "json string cannot be empty")
	}

	if fieldPath == "" {
		return nil, NewCodecError("", j.Name(), "field path cannot be empty")
	}

	var data interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return nil, NewCodecError("", j.Name(),
			fmt.Sprintf("failed to parse json: %v", err))
	}

	return j.extractFieldRecursive(data, fieldPath), nil
}

// extractFieldRecursive 递归提取字段
func (j *JSONCodec) extractFieldRecursive(data interface{}, fieldPath string) interface{} {
	if fieldPath == "" {
		return data
	}

	// 分割字段路径
	parts := j.splitFieldPath(fieldPath)
	if len(parts) == 0 {
		return data
	}

	current := data

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]interface{}:
			if next, exists := v[part]; exists {
				current = next
			} else {
				return nil
			}
		case []interface{}:
			// 处理数组索引
			if index := j.parseArrayIndex(part); index >= 0 && index < len(v) {
				current = v[index]
			} else {
				return nil
			}
		default:
			return nil
		}
	}

	return current
}

// splitFieldPath 分割字段路径
func (j *JSONCodec) splitFieldPath(path string) []string {
	// 简单实现，按点分割
	var parts []string
	current := ""

	for _, char := range path {
		if char == '.' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}

	if current != "" {
		parts = append(parts, current)
	}

	return parts
}

// parseArrayIndex 解析数组索引
func (j *JSONCodec) parseArrayIndex(part string) int {
	var index int
	if _, err := fmt.Sscanf(part, "[%d]", &index); err == nil {
		return index
	}

	if _, err := fmt.Sscanf(part, "%d", &index); err == nil {
		return index
	}

	return -1
}

// Merge 合并两个JSON对象
func (j *JSONCodec) Merge(jsonStr1, jsonStr2 string) (string, error) {
	if jsonStr1 == "" {
		return jsonStr2, nil
	}
	if jsonStr2 == "" {
		return jsonStr1, nil
	}

	var obj1, obj2 map[string]interface{}

	if err := json.Unmarshal([]byte(jsonStr1), &obj1); err != nil {
		return "", NewCodecError("", j.Name(),
			fmt.Sprintf("failed to parse first json: %v", err))
	}

	if err := json.Unmarshal([]byte(jsonStr2), &obj2); err != nil {
		return "", NewCodecError("", j.Name(),
			fmt.Sprintf("failed to parse second json: %v", err))
	}

	merged := j.mergeObjects(obj1, obj2)

	result, err := json.Marshal(merged)
	if err != nil {
		return "", NewCodecError("", j.Name(),
			fmt.Sprintf("failed to marshal merged json: %v", err))
	}

	return string(result), nil
}

// mergeObjects 递归合并对象
func (j *JSONCodec) mergeObjects(obj1, obj2 map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	// 复制第一个对象的所有字段
	for k, v := range obj1 {
		result[k] = v
	}

	// 合并第二个对象的字段
	for k, v := range obj2 {
		if existing, exists := result[k]; exists {
			// 如果两个字段都是对象，递归合并
			if obj1, ok1 := existing.(map[string]interface{}); ok1 {
				if obj2, ok2 := v.(map[string]interface{}); ok2 {
					result[k] = j.mergeObjects(obj1, obj2)
					continue
				}
			}
		}
		// 否则使用第二个对象的值
		result[k] = v
	}

	return result
}

// Filter 过滤JSON对象，只保留指定字段
func (j *JSONCodec) Filter(jsonStr string, fields []string) (string, error) {
	if jsonStr == "" {
		return "", NewCodecError("", j.Name(), "json string cannot be empty")
	}

	if len(fields) == 0 {
		return jsonStr, nil
	}

	var data interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return "", NewCodecError("", j.Name(),
			fmt.Sprintf("failed to parse json: %v", err))
	}

	filtered := j.filterRecursive(data, fields)

	result, err := json.Marshal(filtered)
	if err != nil {
		return "", NewCodecError("", j.Name(),
			fmt.Sprintf("failed to marshal filtered json: %v", err))
	}

	return string(result), nil
}

// filterRecursive 递归过滤
func (j *JSONCodec) filterRecursive(data interface{}, fields []string) interface{} {
	switch v := data.(type) {
	case map[string]interface{}:
		filtered := make(map[string]interface{})
		for _, field := range fields {
			if value, exists := v[field]; exists {
				filtered[field] = value
			}
		}
		return filtered
	case []interface{}:
		filtered := make([]interface{}, len(v))
		for i, item := range v {
			filtered[i] = j.filterRecursive(item, fields)
		}
		return filtered
	default:
		return v
	}
}

// String 返回编解码器的字符串表示
func (j *JSONCodec) String() string {
	return fmt.Sprintf("JSONCodec{name=%s, mime_types=%v}", j.Name(), j.MimeTypes())
}
