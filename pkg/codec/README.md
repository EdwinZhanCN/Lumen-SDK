# Codec Module

## 1. Objective

**Purpose**: 提供数据编解码功能，支持多种数据格式的编码、解码和验证。

**Why needed**: 
- 统一处理不同数据格式的转换（JSON、图像、Base64等）
- 为推理服务提供标准化的数据交换格式
- 确保数据格式兼容性和验证
- 支持自动MIME类型检测和编解码器注册

**Coding Guidelines**:
- 确保所有编解码器实现统一的Codec接口
- 严格验证输入数据的格式和有效性
- 提供详细的错误信息便于调试
- 考虑性能优化，避免不必要的数据拷贝

## 2. Module Structure

```
pkg/codec/
├── codec_registry.go    # 编解码器注册表和管理
├── json_codec.go        # JSON编解码器
├── base64_codec.go      # Base64编解码器
├── image_codec.go       # 图像编解码器
└── README.md           # 本文档
```

**核心组件关系**:
```
CodecRegistry
├── JSONCodec (application/json)
├── Base64Codec (text/plain)
├── ImageCodec (image/*)
└── 支持扩展其他编解码器
```

## 3. Module Members

### Core Interface
- `Codec` - 编解码器标准接口，定义了Encode/Decode/Validate等方法

### Registry
- `CodecRegistry` - 编解码器注册表，支持MIME类型到编解码器的映射
- `GetDefaultRegistry()` - 获取全局默认注册表

### Codecs
- `JSONCodec` - JSON编解码器，支持格式化、压缩、字段提取等
- `Base64Codec` - Base64编解码器，支持标准/URL安全编码
- `ImageCodec` - 图像编解码器，支持JPEG/PNG/GIF格式转换

### Types
- `CodecError` - 编解码器错误类型
- `ImageInfo` - 图像信息描述结构

## 4. Usage

### Basic Usage

```go
// 编码数据
data := map[string]interface{}{
    "message": "Hello World",
    "number":  42,
}

encoded, err := EncodeData("application/json", data)
if err != nil {
    log.Fatal(err)
}

// 解码数据
var decoded map[string]interface{}
err = DecodeData("application/json", encoded, &decoded)
if err != nil {
    log.Fatal(err)
}
```

### Advanced Usage

```go
// 获取编解码器并单独使用
registry := GetDefaultRegistry()
codec, err := registry.Get("application/json")
if err != nil {
    log.Fatal(err)
}

jsonCodec := codec.(*codec.JSONCodec)

// JSON特殊操作
pretty, err := jsonCodec.PrettyPrint(string(encoded))
filtered, err := jsonCodec.Filter(string(encoded), []string{"message"})

// Base64编码
base64Codec := codec.NewBase64CodecWithURLSafe()
encoded, err := base64Codec.EncodeToString(data)
```

### Image Processing

```go
// 图像编解码
imageCodec := codec.NewImageCodecWithOptions(90, "jpeg")

// 编码图像
imageData, err := imageCodec.Encode(image.Image)
if err != nil {
    log.Fatal(err)
}

// 获取图像信息
info, err := imageCodec.GetImageInfo(imageData)
fmt.Printf("Image: %dx%d, format: %s\n", info.Width, info.Height, info.Format)

// Base64图像转换
base64Image, err := imageCodec.ToBase64(imageData, "jpeg")
```

### Custom Codec

```go
// 注册自定义编解码器
type CustomCodec struct {
    // 自定义编解码器实现
}

func (c *CustomCodec) Name() string {
    return "CustomCodec"
}

func (c *CustomCodec) MimeTypes() []string {
    return []string{"application/custom"}
}

func (c *CustomCodec) Encode(data interface{}) ([]byte, error) {
    // 实现编码逻辑
    return nil, nil
}

func (c *CustomCodec) Decode(data []byte, target interface{}) error {
    // 实现解码逻辑
    return nil
}

func (c *CustomCodec) Validate(data interface{}) error {
    // 实现验证逻辑
    return nil
}

// 注册编解码器
customCodec := &CustomCodec{}
err := registry.Register([]string{"application/custom"}, customCodec)
```

### Type Detection

```go
// 自动检测数据类型
data := []byte(`{"message": "hello"}`)
mimeType := DetectType(data)
fmt.Printf("Detected type: %s\n", mimeType)

// 检查支持的类型
if IsSupportedType("application/json") {
    fmt.Println("JSON is supported")
}

// 列出所有支持的类型
types := GetSupportedTypes()
fmt.Printf("Supported types: %v\n", types)
```