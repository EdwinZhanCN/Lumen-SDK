package codec

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"strings"

	_ "image/gif"
)

// ImageCodec 图像编解码器
type ImageCodec struct {
	quality int    // JPEG质量 (1-100)
	format  string // 默认输出格式
}

// NewImageCodec 创建新的图像编解码器
func NewImageCodec() *ImageCodec {
	return &ImageCodec{
		quality: 85,
		format:  "png",
	}
}

// NewImageCodecWithOptions 创建带选项的图像编解码器
func NewImageCodecWithOptions(quality int, format string) *ImageCodec {
	return &ImageCodec{
		quality: quality,
		format:  format,
	}
}

// Name 返回编解码器名称
func (i *ImageCodec) Name() string {
	return "ImageCodec"
}

// MimeTypes 返回支持的MIME类型列表
func (i *ImageCodec) MimeTypes() []string {
	return []string{
		"image/jpeg",
		"image/jpg",
		"image/png",
		"image/gif",
	}
}

// Encode 将图像编码为字节数据
func (i *ImageCodec) Encode(data interface{}) ([]byte, error) {
	img, err := i.toImage(data)
	if err != nil {
		return nil, NewCodecError("", i.Name(),
			fmt.Sprintf("failed to convert data to image: %v", err))
	}

	var buf bytes.Buffer

	// 根据格式编码
	switch strings.ToLower(i.format) {
	case "jpeg", "jpg":
		err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: i.quality})
	case "png":
		err = png.Encode(&buf, img)
	case "gif":
		// GIF编码需要动画帧，这里简化处理
		err = i.encodeGeneric(&buf, img, "gif")
	default:
		// 默认使用PNG
		err = png.Encode(&buf, img)
	}

	if err != nil {
		return nil, NewCodecError("", i.Name(),
			fmt.Sprintf("failed to encode image: %v", err))
	}

	return buf.Bytes(), nil
}

// EncodeToFormat 将图像编码为指定格式
func (i *ImageCodec) EncodeToFormat(data interface{}, format string) ([]byte, error) {
	originalFormat := i.format
	i.format = format
	defer func() { i.format = originalFormat }()

	return i.Encode(data)
}

// Decode 将字节数据解码为图像
func (i *ImageCodec) Decode(data []byte, target interface{}) error {
	if len(data) == 0 {
		return NewCodecError("", i.Name(), "image data cannot be empty")
	}

	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return NewCodecError("", i.Name(),
			fmt.Sprintf("failed to decode image: %v", err))
	}

	// 根据target类型设置解码结果
	switch t := target.(type) {
	case *image.Image:
		*t = img
	case *[]byte:
		*t = data
	case *string:
		*t = base64.StdEncoding.EncodeToString(data)
	case **image.Image:
		*t = &img
	case *map[string]interface{}:
		if t == nil {
			return NewCodecError("", i.Name(), "target map pointer cannot be nil")
		}
		if *t == nil {
			*t = make(map[string]interface{})
		}
		(*t)["image"] = img
		(*t)["format"] = format
		(*t)["data"] = data
		(*t)["size"] = len(data)
		(*t)["base64"] = base64.StdEncoding.EncodeToString(data)
	default:
		return NewCodecError("", i.Name(),
			fmt.Sprintf("unsupported target type: %T", target))
	}

	return nil
}

// Validate 验证数据是否为有效图像
func (i *ImageCodec) Validate(data interface{}) error {
	if data == nil {
		return NewCodecError("", i.Name(), "data cannot be nil")
	}

	switch v := data.(type) {
	case []byte:
		if len(v) == 0 {
			return NewCodecError("", i.Name(), "image data cannot be empty")
		}
		// 尝试解码以验证
		_, _, err := image.Decode(bytes.NewReader(v))
		if err != nil {
			return NewCodecError("", i.Name(),
				fmt.Sprintf("invalid image data: %v", err))
		}
	case string:
		if v == "" {
			return NewCodecError("", i.Name(), "image string cannot be empty")
		}
		// 检查是否为base64
		if strings.HasPrefix(v, "data:image/") {
			// Base64编码的图像
			data, err := i.decodeBase64Image(v)
			if err != nil {
				return NewCodecError("", i.Name(),
					fmt.Sprintf("invalid base64 image: %v", err))
			}
			return i.Validate(data)
		}
		// 尝试解码为base64
		data, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			return NewCodecError("", i.Name(),
				fmt.Sprintf("invalid base64 string: %v", err))
		}
		return i.Validate(data)
	case image.Image:
		// 检查图像尺寸
		bounds := v.Bounds()
		if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
			return NewCodecError("", i.Name(),
				"image has invalid dimensions")
		}
	default:
		return NewCodecError("", i.Name(),
			fmt.Sprintf("unsupported data type: %T", data))
	}

	return nil
}

// DetectFormat 检测图像格式
func (i *ImageCodec) DetectFormat(data []byte) (string, error) {
	if len(data) == 0 {
		return "", NewCodecError("", i.Name(), "data cannot be empty")
	}

	_, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", NewCodecError("", i.Name(),
			fmt.Sprintf("failed to detect image format: %v", err))
	}

	return format, nil
}

// GetImageInfo 获取图像信息
func (i *ImageCodec) GetImageInfo(data []byte) (*ImageInfo, error) {
	if len(data) == 0 {
		return nil, NewCodecError("", i.Name(), "data cannot be empty")
	}

	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, NewCodecError("", i.Name(),
			fmt.Sprintf("failed to decode image: %v", err))
	}

	bounds := img.Bounds()
	info := &ImageInfo{
		Format:     format,
		Width:      bounds.Dx(),
		Height:     bounds.Dy(),
		ColorModel: fmt.Sprintf("%T", img.ColorModel()),
		Size:       len(data),
		Bounds:     bounds,
	}

	// 计算宽高比
	if info.Height > 0 {
		info.AspectRatio = float64(info.Width) / float64(info.Height)
	}

	return info, nil
}

// Resize 调整图像尺寸
func (i *ImageCodec) Resize(data []byte, width, height int) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, NewCodecError("", i.Name(),
			fmt.Sprintf("failed to decode image: %v", err))
	}

	// 简单的最近邻插值缩放
	resized := i.resizeImage(img, width, height)

	// 编码缩放后的图像
	return i.Encode(resized)
}

// Crop 裁剪图像
func (i *ImageCodec) Crop(data []byte, x, y, width, height int) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, NewCodecError("", i.Name(),
			fmt.Sprintf("failed to decode image: %v", err))
	}

	bounds := img.Bounds()

	// 验证裁剪区域
	if x < bounds.Min.X || y < bounds.Min.Y ||
		x+width > bounds.Max.X || y+height > bounds.Max.Y {
		return nil, NewCodecError("", i.Name(),
			"crop region is outside image bounds")
	}

	// 创建裁剪后的图像
	cropped := image.NewRGBA(image.Rect(x, y, x+width, y+height))
	for dy := 0; dy < height; dy++ {
		for dx := 0; dx < width; dx++ {
			cropped.Set(dx, dy, img.At(x+dx, y+dy))
		}
	}

	return i.Encode(cropped)
}

// ToBase64 将图像转换为Base64字符串
func (i *ImageCodec) ToBase64(data []byte, format string) (string, error) {
	if len(data) == 0 {
		return "", NewCodecError("", i.Name(), "data cannot be empty")
	}

	if format == "" {
		detectedFormat, err := i.DetectFormat(data)
		if err != nil {
			return "", err
		}
		format = detectedFormat
	}

	base64Str := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("data:image/%s;base64,%s", format, base64Str), nil
}

// FromBase64 从Base64字符串解码图像
func (i *ImageCodec) FromBase64(base64Str string) ([]byte, error) {
	if base64Str == "" {
		return nil, NewCodecError("", i.Name(), "base64 string cannot be empty")
	}

	return i.decodeBase64Image(base64Str)
}

// 辅助方法

// toImage 将各种数据类型转换为image.Image
func (i *ImageCodec) toImage(data interface{}) (image.Image, error) {
	switch v := data.(type) {
	case image.Image:
		return v, nil
	case []byte:
		if len(v) == 0 {
			return nil, fmt.Errorf("image data is empty")
		}
		img, _, err := image.Decode(bytes.NewReader(v))
		if err != nil {
			return nil, fmt.Errorf("failed to decode image: %v", err)
		}
		return img, nil
	case string:
		if v == "" {
			return nil, fmt.Errorf("image string is empty")
		}
		// 检查是否为data URL
		if strings.HasPrefix(v, "data:image/") {
			data, err := i.decodeBase64Image(v)
			if err != nil {
				return nil, fmt.Errorf("failed to decode base64 image: %v", err)
			}
			return i.toImage(data)
		}
		// 尝试解码为base64
		data, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			return nil, fmt.Errorf("failed to decode base64 string: %v", err)
		}
		return i.toImage(data)
	default:
		return nil, fmt.Errorf("unsupported data type: %T", data)
	}
}

// decodeBase64Image 解码Base64图像
func (i *ImageCodec) decodeBase64Image(base64Str string) ([]byte, error) {
	// 提取Base64部分
	if strings.HasPrefix(base64Str, "data:image/") {
		parts := strings.SplitN(base64Str, ",", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid data URL format")
		}
		base64Str = parts[1]
	}

	return base64.StdEncoding.DecodeString(base64Str)
}

// encodeGeneric 通用编码方法
func (i *ImageCodec) encodeGeneric(w io.Writer, img image.Image, format string) error {
	// 简化实现，实际应该根据format使用对应的编码器
	return png.Encode(w, img)
}

// resizeImage 调整图像尺寸（简单实现）
func (i *ImageCodec) resizeImage(img image.Image, width, height int) image.Image {
	srcBounds := img.Bounds()
	srcW, srcH := srcBounds.Dx(), srcBounds.Dy()

	dst := image.NewRGBA(image.Rect(0, 0, width, height))

	// 简单的最近邻插值
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			srcX := x * srcW / width
			srcY := y * srcH / height
			dst.Set(x, y, img.At(srcX, srcY))
		}
	}

	return dst
}

// SetQuality 设置JPEG质量
func (i *ImageCodec) SetQuality(quality int) {
	if quality < 1 {
		quality = 1
	} else if quality > 100 {
		quality = 100
	}
	i.quality = quality
}

// SetFormat 设置默认输出格式
func (i *ImageCodec) SetFormat(format string) {
	i.format = format
}

// GetQuality 获取JPEG质量
func (i *ImageCodec) GetQuality() int {
	return i.quality
}

// GetFormat 获取默认输出格式
func (i *ImageCodec) GetFormat() string {
	return i.format
}

// ImageInfo 图像信息结构
type ImageInfo struct {
	Format      string          `json:"format"`
	Width       int             `json:"width"`
	Height      int             `json:"height"`
	ColorModel  string          `json:"color_model"`
	Size        int             `json:"size"`
	Bounds      image.Rectangle `json:"bounds"`
	AspectRatio float64         `json:"aspect_ratio"`
}

// String 返回图像信息的字符串表示
func (info *ImageInfo) String() string {
	if info == nil {
		return "nil"
	}
	return fmt.Sprintf("ImageInfo{format=%s, size=%dx%d, bytes=%d}",
		info.Format, info.Width, info.Height, info.Size)
}

// String 返回编解码器的字符串表示
func (i *ImageCodec) String() string {
	return fmt.Sprintf("ImageCodec{name=%s, quality=%d, format=%s, mime_types=%v}",
		i.Name(), i.quality, i.format, i.MimeTypes())
}
