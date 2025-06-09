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
		AppName:           "Go Downloader",
		ErrorHandler:      utils.CustomErrorHandler,
		StreamRequestBody: true, // Enable streaming for better memory usage
		Prefork:           false,
		ProxyHeader:       "X-Forwarded-For",
		BodyLimit:         512 * 1024 * 1024, // this is the default limit of 512MB
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
	app.Use(recover.New())
	app.Use(etag.New())
	app.Use(earlydata.New())
	app.Use(idempotency.New())
	app.Use(helmet.New())
	app.Use(cors.New(cors.Config{AllowOrigins: "*", AllowMethods: "*", AllowHeaders: "*"}))
	// Use moderate compression for better performance balance
	app.Use(compress.New(compress.Config{
		Level: compress.LevelDefault, // Better performance than LevelBestCompression
		Next: func(c *fiber.Ctx) bool {
			// Skip compression for zip endpoints as they're already compressed
			return strings.HasPrefix(c.Path(), "/zip/")
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

	app.Post("/zip/multi", controllers.ZipMultipleFiles)
	// Alternative high-performance endpoint
	app.Post("/zip/multi/optimized", controllers.ZipMultipleFilesOptimized)
	// Performance monitoring endpoint
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

	log.Printf("Started server on: %s:%s\n", HOST, PORT)
	err = app.Listen(HOST + ":" + PORT)

	if err != nil {
		_ = minioClients.Storage.Close()
		log.Println("Failed to start server")
		panic(err)
	}
}
