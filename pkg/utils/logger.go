package utils

import (
	"os"
	"strings"

	"github.com/edwinzhancn/lumen-sdk/pkg/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Logger is the global structured logger instance for the Lumen SDK.
//
// This is a zap.Logger configured based on the logging configuration. Use this
// for performance-critical logging with structured fields.
var Logger *zap.Logger

// Sugar is the global sugared logger for more convenient logging.
//
// SugaredLogger provides a more ergonomic API with printf-style formatting
// at the cost of minor performance overhead. Use for non-critical paths.
var Sugar *zap.SugaredLogger

// InitLogger initializes the global logger with the specified configuration.
//
// This function sets up structured logging with configurable:
//   - Log level (debug, info, warn, error, fatal)
//   - Output format (json, text/console)
//   - Output destination (stdout, stderr, file)
//   - File rotation (when using file output)
//
// The function initializes both Logger (structured) and Sugar (convenient) loggers.
// It should be called once during application startup.
//
// Parameters:
//   - cfg: Logging configuration specifying level, format, and output
//
// Role in project: Configures the centralized logging system used throughout the SDK.
// Proper logging is essential for debugging, monitoring, and operational visibility.
//
// Example:
//
//	// Initialize with custom config
//	logCfg := &config.LoggingConfig{
//	    Level:  "debug",
//	    Format: "json",
//	    Output: "stdout",
//	}
//	utils.InitLogger(logCfg)
//
//	// Use the logger
//	utils.Logger.Info("Client started",
//	    zap.String("version", "1.0.0"),
//	    zap.Int("nodes", 3))
//
//	// Or use sugar for convenience
//	utils.Sugar.Infof("Client started: version=%s, nodes=%d", "1.0.0", 3)
func InitLogger(cfg *config.LoggingConfig) {
	// 解析日志级别
	level := parseLogLevel(cfg.Level)

	// 创建编码器配置
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "message",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// 根据格式选择编码器
	var encoder zapcore.Encoder
	if cfg.Format == "json" {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	// 设置输出目标
	var writeSyncer zapcore.WriteSyncer
	if strings.HasPrefix(cfg.Output, "file:") {
		filePath := strings.TrimPrefix(cfg.Output, "file:")
		writeSyncer = zapcore.AddSync(&lumberjack.Logger{
			Filename:   filePath,
			MaxSize:    100,
			MaxBackups: 3,
			MaxAge:     28,
			Compress:   true,
		})
	} else {
		switch strings.ToLower(cfg.Output) {
		case "stderr":
			writeSyncer = zapcore.AddSync(os.Stderr)
		case "stdout", "":
			writeSyncer = zapcore.AddSync(os.Stdout)
		default:
			writeSyncer = zapcore.AddSync(os.Stdout)
		}
	}

	// 创建核心和 logger
	core := zapcore.NewCore(encoder, writeSyncer, level)
	Logger = zap.New(core, zap.AddCaller())
	Sugar = Logger.Sugar()
}

// parseLogLevel 解析日志级别
func parseLogLevel(level string) zapcore.Level {
	switch strings.ToLower(level) {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn", "warning":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	case "fatal":
		return zapcore.FatalLevel
	default:
		return zapcore.InfoLevel
	}
}

// 如果没有初始化，使用默认配置
func init() {
	InitLogger(&config.LoggingConfig{
		Level:  "info",
		Format: "json",
		Output: "stdout",
	})
}
