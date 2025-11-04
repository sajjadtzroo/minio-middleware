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
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/joho/godotenv"
)

const PORT = "3000"
const HOST = "0.0.0.0"

func main() {
	// Try to load .env but don't fail if it doesn't exist
	if err := godotenv.Load(); err != nil {
		log.Printf("⚠️ Warning: .env file not found, using environment variables")
	}

	// Check for critical environment variables (only the absolutely required ones)
	requiredEnvs := []string{
		"MINIO_ENDPOINT",
		"MINIO_ACCESSKEY",
		"MINIO_SECRETKEY",
		"JWT_KEY",
	}

	missingEnvs := []string{}
	for _, env := range requiredEnvs {
		if os.Getenv(env) == "" {
			missingEnvs = append(missingEnvs, env)
		}
	}

	if len(missingEnvs) > 0 {
		log.Printf("❌ Missing required environment variables: %v", missingEnvs)
		log.Printf("📝 Please set these environment variables or create a .env file")
		log.Fatal("Cannot start without required configuration")
	}

	// Optional environment variables - set defaults if not provided
	if os.Getenv("DEST_CHAT_ID") == "" {
		log.Printf("⚠️ DEST_CHAT_ID not set, using default")
		os.Setenv("DEST_CHAT_ID", "-1001234567890")
	}

	// Check for bot tokens - at least one should be present
	botTokens := []string{
		"BOT_TELEGRAM",
		"BOT_INSTAGRAM",
		"BOT_TRACKER",
		"BOT_INFLUENCER",
	}

	hasAnyBot := false
	for _, botEnv := range botTokens {
		if os.Getenv(botEnv) != "" {
			hasAnyBot = true
			log.Printf("✅ Found %s configuration", botEnv)
		} else {
			log.Printf("⚠️ %s not configured, scope will be empty", botEnv)
		}
	}

	if !hasAnyBot {
		log.Printf("⚠️ Warning: No bot tokens configured, most features will not work!")
	}

	log.Printf("🔧 Starting server initialization...")

	// Initiate MinIO
	var minioClients config.MinIOClients
	minioConfig := config.GetMinioCredentials()

	// Try to initialize MinIO, but continue if it fails (for development)
	defer func() {
		if r := recover(); r != nil {
			log.Printf("❌ MinIO initialization failed: %v", r)
			log.Printf("⚠️ Starting without MinIO support - storage features will not work")
			// Create a dummy MinIO client
			minioClients = config.MinIOClients{}
		}
	}()

	minioClients = config.GetMinIOClients(minioConfig)
	log.Printf("✅ MinIO client initialized")

	// Initialize Snitch configuration (optional)
	snitchConfiguration := config.NewSnitchConfiguration()
	log.Printf("✅ Snitch configuration loaded")

	// Initialize bot scope configuration
	botScopeConfig := config.NewBotScopeConfiguration()
	allScopes := botScopeConfig.GetAllScopes()
	if len(allScopes) > 0 {
		log.Printf("✅ Bot scopes initialized: %v", allScopes)
	} else {
		log.Printf("⚠️ No bot scopes available")
	}

	// Initialize Instagram API (optional)
	instagramApi := instagram_api.New(os.Getenv("INSTAGRAM_API"))
	if os.Getenv("INSTAGRAM_API") != "" {
		log.Printf("✅ Instagram API initialized")
	} else {
		log.Printf("⚠️ Instagram API token not provided")
	}

	// Create Fiber app
	app := fiber.New(fiber.Config{
		AppName:           "Go Downloader",
		ErrorHandler:      utils.CustomErrorHandler,
		StreamRequestBody: true, // Enable streaming for better memory usage
		Prefork:           false,
		ProxyHeader:       "X-Forwarded-For",
		BodyLimit:         512 * 1024 * 1024, // 512MB limit
		ReadBufferSize:    8192,              // Optimize read buffer size
		WriteBufferSize:   8192,              // Optimize write buffer size
		Network:           "tcp",
		EnablePrintRoutes: false,
		DisableKeepalive:  false,             // Keep connections alive for better performance
		ReadTimeout:       60 * time.Second,  // Increased timeout for large files
		WriteTimeout:      300 * time.Second, // Increased timeout for ZIP creation
		IdleTimeout:       120 * time.Second,
	})

	// Middlewares
	app.Use(recover.New(recover.Config{
		EnableStackTrace: true,
	}))
	app.Use(etag.New())
	app.Use(earlydata.New())
	app.Use(idempotency.New())
	app.Use(helmet.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "*",
		AllowHeaders: "*",
	}))

	// Use moderate compression for better performance balance
	app.Use(compress.New(compress.Config{
		Level: compress.LevelDefault, // Better performance than LevelBestCompression
		Next: func(c *fiber.Ctx) bool {
			// Skip compression for zip endpoints as they're already compressed
			return strings.HasPrefix(c.Path(), "/zip/")
		},
	}))

	// JWT Middleware
	JWTMiddleware := middleware.Authentication

	// Attach MinIO clients
	app.Use(middleware.Attach(&minioClients))

	// Attach other configurations
	app.Use(func(ctx *fiber.Ctx) error {
		// Set the bot scope configuration
		ctx.Locals("BOT_SCOPE_CONFIG", botScopeConfig)
		ctx.Locals("INSTAGRAM_API", instagramApi)
		ctx.Locals("SNITCH_CONFIG", snitchConfiguration)
		return ctx.Next()
	})

	// Health check endpoint
	app.Get("/health", func(c *fiber.Ctx) error {
		health := fiber.Map{
			"status":    "healthy",
			"timestamp": time.Now().Unix(),
			"version":   "1.0.0",
		}

		// Check MinIO connection
		if minioClients.Storage != nil && minioClients.Storage.Conn() != nil {
			health["storage"] = "connected"
		} else {
			health["storage"] = "disconnected"
		}

		// Check bot scopes
		scopes := botScopeConfig.GetAllScopes()
		health["bot_scopes"] = len(scopes)
		health["available_scopes"] = scopes

		return c.JSON(health)
	})

	// Status endpoint
	app.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"result":  true,
			"message": "Go Uploader Service is running",
			"version": "1.0.0",
			"endpoints": []string{
				"/health",
				"/bot-scopes",
				"/upload/telegram/:botName",
				"/instant/:botName/:fileId",
				"/profile/:media/:pk/:userName",
			},
		})
	})

	// Register routes
	// ZIP operations
	app.Post("/zip/multi", controllers.ZipMultipleFiles)
	app.Post("/zip/multi/optimized", controllers.ZipMultipleFilesOptimized)
	app.Get("/zip/performance", controllers.GetZipPerformanceInfo)

	// Telegram upload operations
	app.Post("/upload/telegram/link/:botName", controllers.UploadToTelegramViaLink)
	app.Post("/upload/telegram/link/:botName", JWTMiddleware, controllers.UploadToTelegramViaLink)
	app.Post("/upload/telegram/:botName", JWTMiddleware, controllers.UploadToTelegram)
	app.Post("/upload/telegram/:botName/:specificBot", JWTMiddleware, controllers.UploadToTelegram)

	// Transfer operations
	app.Post("/transfer/telegram", controllers.TransferFileId)

	// Profile operations
	app.Get("/profile/:media/:pk/:userName", controllers.DownloadProfile)

	// Instant operations
	app.Post("/instant/link", controllers.DownloadFromLinkAndUpload)
	app.Get("/instant/:botName/:fileId", controllers.DownloadFromTelegram)
	app.Get("/instant/:botName/:fileId/:specificBot", controllers.DownloadFromTelegram)

	// Direct storage operations
	app.Post("/direct/:bucketName", JWTMiddleware, controllers.UploadFile)
	app.Get("/direct/*", controllers.DownloadFile)

	// Bot scope management
	app.Get("/bot-scopes", controllers.ListBotScopes)

	// 404 handler
	app.Use(func(c *fiber.Ctx) error {
		return c.Status(404).JSON(fiber.Map{
			"result":  false,
			"message": "Endpoint not found",
			"path":    c.Path(),
		})
	})

	// Print startup information
	log.Printf("========================================")
	log.Printf("🚀 Go Uploader Service")
	log.Printf("========================================")
	log.Printf("📍 Server: %s:%s", HOST, PORT)
	log.Printf("🔐 JWT Auth: %v", os.Getenv("JWT_KEY") != "")
	log.Printf("💾 MinIO: %s", os.Getenv("MINIO_ENDPOINT"))
	log.Printf("🤖 Bot Scopes: %v", allScopes)
	log.Printf("========================================")
	log.Printf("✅ Server starting on http://%s:%s", HOST, PORT)
	log.Printf("========================================")

	// Start server
	if err := app.Listen(HOST + ":" + PORT); err != nil {
		if minioClients.Storage != nil {
			_ = minioClients.Storage.Close()
		}
		log.Fatal("Failed to start server: ", err)
	}
}