package rest

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"go.uber.org/zap"
)

// Router 路由器
type Router struct {
	handler *Handler
	app     *fiber.App
	logger  *zap.Logger
}

// NewRouter 创建新的路由器
func NewRouter(handler *Handler, logger *zap.Logger) *Router {
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}

			errorResp := APIResponse{
				Success: false,
				Error: &APIError{
					Code:    "internal_error",
					Message: err.Error(),
				},
				RequestID: c.GetRespHeader("X-Request-ID"),
				Timestamp: time.Now(),
			}

			return c.Status(code).JSON(errorResp)
		},
	})

	// 添加中间件
	app.Use(recover.New())
	app.Use(requestid.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization, X-Request-ID",
	}))

	return &Router{
		handler: handler,
		app:     app,
		logger:  logger,
	}
}

// SetupRoutes 设置路由
func (r *Router) SetupRoutes() {
	// API版本1
	v1 := r.app.Group("/api/v1")

	// 推理API
	v1.Post("/embed", r.handler.HandleEmbed)
	v1.Post("/detect", r.handler.HandleDetect)
	v1.Post("/ocr", r.handler.HandleOCR)
	v1.Post("/tts", r.handler.HandleTTS)

	// 管理API
	v1.Get("/nodes", r.handler.HandleNodes)
	v1.Get("/health", r.handler.HandleHealth)
	v1.Get("/metrics", r.handler.HandleMetrics)

	// 根路径
	r.app.Get("/", r.handleRoot)
	r.app.Get("/health", r.handler.HandleHealth)

	r.logger.Info("routes configured successfully")
}

// handleRoot 处理根路径请求
func (r *Router) handleRoot(c *fiber.Ctx) error {
	return c.JSON(map[string]interface{}{
		"service":     "Lumen Hub",
		"version":     "1.0.0",
		"description": "Lumen AI Services Hub",
		"endpoints": map[string]string{
			"api_docs": "/api/v1",
			"mcp":      "/mcp/v1",
			"health":   "/health",
		},
	})
}

// GetApp 获取Fiber应用实例
func (r *Router) GetApp() *fiber.App {
	return r.app
}

// Start 启动服务器
func (r *Router) Start(addr string) error {
	r.logger.Info("starting REST API server",
		zap.String("address", addr))
	return r.app.Listen(addr)
}

// StartTLS 启动HTTPS服务器
func (r *Router) StartTLS(addr, certFile, keyFile string) error {
	r.logger.Info("starting REST API server with TLS",
		zap.String("address", addr),
		zap.String("cert_file", certFile))
	return r.app.ListenTLS(addr, certFile, keyFile)
}
