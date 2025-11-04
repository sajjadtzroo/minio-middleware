package main

import (
	"go-uploader/config"
	"go-uploader/controllers"
	"go-uploader/middleware"
	"go-uploader/pkg/instagram_api"
	"go-uploader/utils"
	"log"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/earlydata"
	"github.com/gofiber/fiber/v2/middleware/etag"
	"github.com/gofiber/fiber/v2/middleware/helmet"
	"github.com/gofiber/fiber/v2/middleware/idempotency"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/joho/godotenv"
)

const PORT = "3000"
const HOST = "0.0.0.0"

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// Initiate
	minioConfig := config.GetMinioCredentials()
	minioClients := config.GetMinIOClients(minioConfig)

	snitchConfiguration := config.NewSnitchConfiguration()

	// Initialize bot scope configuration
	botScopeConfig := config.NewBotScopeConfiguration()

	instagramApi := instagram_api.New(os.Getenv("INSTAGRAM_API"))

	app := fiber.New(fiber.Config{
		AppName:           "Go Downloader v1.2.0",
		ErrorHandler:      utils.CustomErrorHandler,
		StreamRequestBody: true, // Enable streaming for better memory usage
		Prefork:           false,
		ProxyHeader:       "X-Forwarded-For",
		BodyLimit:         512 * 1024 * 1024, // 512MB limit
		ReadBufferSize:    16384,             // Increased from 8192
		WriteBufferSize:   16384,             // Increased from 8192
		Network:           "tcp",
		EnablePrintRoutes: false,
		DisableKeepalive:  false,             // Keep connections alive for better performance
		ReadTimeout:       60 * time.Second,  // Increased timeout for large files
		WriteTimeout:      300 * time.Second, // Increased timeout for ZIP creation
		IdleTimeout:       120 * time.Second,
	})

	// Middlewares
	app.Use(recover.New())
	app.Use(etag.New())
	app.Use(earlydata.New())
	app.Use(idempotency.New())
	app.Use(helmet.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "*",
		AllowHeaders: "*",
		MaxAge:       3600,
	}))

	// Rate limiting - important for preventing abuse
	app.Use(limiter.New(limiter.Config{
		Max:               100,
		Expiration:        1 * time.Minute,
		LimiterMiddleware: limiter.SlidingWindow{},
		SkipFailedRequests: false,
		SkipSuccessfulRequests: false,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP() // Rate limit by IP
		},
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"result":  false,
				"message": "Too many requests, please try again later",
			})
		},
	}))

	// Use moderate compression for better performance balance
	app.Use(compress.New(compress.Config{
		Level: compress.LevelDefault, // Better performance than LevelBestCompression
		Next: func(c *fiber.Ctx) bool {
			// Skip compression for zip endpoints and streaming responses
			path := c.Path()
			return strings.HasPrefix(path, "/zip/") ||
				   strings.HasPrefix(path, "/instant/") ||
				   strings.Contains(c.Get("Accept-Encoding"), "identity")
		},
	}))

	JWTMiddleware := middleware.Authentication

	app.Use(middleware.Attach(&minioClients))
	app.Use(func(ctx *fiber.Ctx) error {
		// Set the bot scope configuration - contains all bot arrays in hashmap
		ctx.Locals("BOT_SCOPE_CONFIG", botScopeConfig)
		ctx.Locals("INSTAGRAM_API", instagramApi)
		ctx.Locals("SNITCH_CONFIG", snitchConfiguration)
		return ctx.Next()
	})

	// Health check endpoint
	app.Get("/health", func(c *fiber.Ctx) error {
		botScopes := botScopeConfig.GetAllScopes()
		return c.JSON(fiber.Map{
			"status":     "healthy",
			"timestamp":  time.Now().Unix(),
			"bot_scopes": botScopes,
			"version":    "1.2.0",
			"cache":      "enabled",
			"features": fiber.Map{
				"cache_enabled":    true,
				"bot_racing":       true,
				"named_bots":       true,
				"rate_limiting":    true,
				"compression":      true,
				"optimized_racing": true,
			},
		})
	})

	// Status endpoint with more details
	app.Get("/status", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"result": true,
			"server": fiber.Map{
				"host":    HOST,
				"port":    PORT,
				"version": "1.2.0",
			},
			"performance": fiber.Map{
				"cache_enabled":       true,
				"optimized_timeouts":  true,
				"connection_pooling":  true,
				"rate_limiting":       true,
				"compression_enabled": true,
			},
		})
	})

	// Favicon handler
	app.Get("/favicon.ico", func(c *fiber.Ctx) error {
		return c.SendStatus(204)
	})

	app.Post("/zip/multi", controllers.ZipMultipleFiles)
	app.Post("/zip/multi/optimized", controllers.ZipMultipleFilesOptimized)
	app.Get("/zip/performance", controllers.GetZipPerformanceInfo)

	app.Post("/upload/telegram/link/:botName", controllers.UploadToTelegramViaLink)
	app.Post("/upload/telegram/link/:botName", JWTMiddleware, controllers.UploadToTelegramViaLink)
	app.Post("/upload/telegram/:botName", JWTMiddleware, controllers.UploadToTelegram)
	app.Post("/upload/telegram/:botName/:specificBot", JWTMiddleware, controllers.UploadToTelegram)

	app.Post("/transfer/telegram", controllers.TransferFileId)

	app.Get("/profile/:media/:pk/:userName", controllers.DownloadProfile)

	app.Post("/instant/link", controllers.DownloadFromLinkAndUpload)
	app.Get("/instant/:botName/:fileId", controllers.DownloadFromTelegram)
	app.Get("/instant/:botName/:fileId/:specificBot", controllers.DownloadFromTelegram)

	app.Post("/direct/:bucketName", JWTMiddleware, controllers.UploadFile)
	app.Get("/direct/*", controllers.DownloadFile)

	// Bot scope management
	app.Get("/bot-scopes", controllers.ListBotScopes)

	log.Printf("=====================================")
	log.Printf("🚀 Server starting on: %s:%s", HOST, PORT)
	log.Printf("✅ Cache: ENABLED")
	log.Printf("✅ Bot Racing: OPTIMIZED")
	log.Printf("✅ Rate Limiting: ENABLED")
	log.Printf("✅ Connection Pooling: ENABLED")
	log.Printf("✅ Version: 1.2.0")
	log.Printf("=====================================")

	err = app.Listen(HOST + ":" + PORT)

	if err != nil {
		_ = minioClients.Storage.Close()
		log.Println("Failed to start server")
		panic(err)
	}
}