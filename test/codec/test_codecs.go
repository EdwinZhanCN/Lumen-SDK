package main

import (
	"fmt"
	"log"

	"github.com/edwinzhancn/lumen-sdk/pkg/codec"
)

func main() {
	fmt.Println("Testing Lumen SDK Codec Package...")

	// 测试默认注册表
	registry := codec.GetDefaultRegistry()

	// 列出所有支持的MIME类型
	mimeTypes := registry.List()
	fmt.Printf("Supported MIME types: %v\n", mimeTypes)

	// 测试JSON编解码器
	fmt.Println("\n=== Testing JSON Codec ===")
	testJSONCodec(registry)

	// 测试Base64编解码器
	fmt.Println("\n=== Testing Base64 Codec ===")
	testBase64Codec(registry)

	// 测试图像编解码器
	fmt.Println("\n=== Testing Image Codec ===")
	testImageCodec(registry)

	// 测试编解码器注册表功能
	fmt.Println("\n=== Testing Codec Registry ===")
	testCodecRegistry(registry)

	fmt.Println("\nAll tests completed successfully!")
}

func testJSONCodec(registry *codec.CodecRegistry) {
	testData := map[string]interface{}{
		"name":     "Lumen SDK",
		"version":  "1.0.0",
		"features": []string{"codec", "registry", "json", "base64", "image"},
	}

	// 编码
	encoded, err := registry.Encode("application/json", testData)
	if err != nil {
		log.Printf("JSON encoding failed: %v", err)
		return
	}
	fmt.Printf("JSON encoded: %s\n", string(encoded))

	// 解码
	var decoded map[string]interface{}
	err = registry.Decode("application/json", encoded, &decoded)
	if err != nil {
		log.Printf("JSON decoding failed: %v", err)
		return
	}
	fmt.Printf("JSON decoded: %+v\n", decoded)
}

func testBase64Codec(registry *codec.CodecRegistry) {
	testData := "Hello, Lumen SDK!"

	// 编码
	encoded, err := registry.Encode("text/plain", testData)
	if err != nil {
		log.Printf("Base64 encoding failed: %v", err)
		return
	}
	fmt.Printf("Base64 encoded: %s\n", string(encoded))

	// 解码到字符串
	var decoded string
	err = registry.Decode("text/plain", encoded, &decoded)
	if err != nil {
		log.Printf("Base64 decoding failed: %v", err)
		return
	}
	fmt.Printf("Base64 decoded: %s\n", decoded)
}

func testImageCodec(registry *codec.CodecRegistry) {
	// 创建一个简单的1x1红色PNG图像数据
	// 这是一个最小化的PNG文件头，代表一个1x1的红色像素
	simplePNG := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, // PNG signature
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52, // IHDR chunk start
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, // 1x1 dimensions
		0x08, 0x02, 0x00, 0x00, 0x00, // bit depth, color type, compression, filter, interlace
		0x90, 0x77, 0x53, 0xDE, // CRC
		0x00, 0x00, 0x00, 0x0C, 0x49, 0x44, 0x41, 0x54, // IDAT chunk start
		0x08, 0x99, 0x01, 0x01, 0x00, 0x00, 0x00, 0xFF, 0xFF, 0x00, 0x00, 0x00, 0x02, 0x00, 0x01, // image data
		0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82, // IEND chunk
	}

	fmt.Printf("Testing with %d bytes of PNG data\n", len(simplePNG))

	// 检测图像格式
	detectedFormat := codec.DetectType(simplePNG)
	fmt.Printf("Detected format: %s\n", detectedFormat)

	// 获取图像信息
	if detectedFormat == "image/png" {
		imageCodec := codec.NewImageCodec()
		info, err := imageCodec.GetImageInfo(simplePNG)
		if err != nil {
			log.Printf("Failed to get image info: %v", err)
		} else {
			fmt.Printf("Image info: %s\n", info.String())
		}
	}
}

func testCodecRegistry(registry *codec.CodecRegistry) {
	// 获取注册表统计信息
	stats := registry.GetStats()
	fmt.Printf("Registry stats: %+v\n", stats)

	// 检查特定编解码器是否存在
	jsonExists := registry.Exists("application/json")
	fmt.Printf("JSON codec exists: %t\n", jsonExists)

	// 按名称获取编解码器
	jsonCodec, err := registry.GetCodecByName("JSONCodec")
	if err != nil {
		log.Printf("Failed to get JSON codec by name: %v", err)
	} else {
		fmt.Printf("JSON codec found: %s\n", jsonCodec.Name())
	}

	// 测试数据验证
	testData := map[string]interface{}{"test": "data"}
	err = registry.Validate("application/json", testData)
	if err != nil {
		log.Printf("JSON validation failed: %v", err)
	} else {
		fmt.Println("JSON validation passed")
	}
}
