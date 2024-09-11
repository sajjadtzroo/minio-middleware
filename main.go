package main

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/earlydata"
	"github.com/gofiber/fiber/v2/middleware/etag"
	"github.com/gofiber/fiber/v2/middleware/helmet"
	"github.com/gofiber/fiber/v2/middleware/idempotency"
	"github.com/joho/godotenv"
	"go-uploader/config"
	"go-uploader/controllers"
	"go-uploader/middleware"
	"go-uploader/pkg/instagram_api"
	"go-uploader/pkg/telegram_api"
	"go-uploader/utils"
	"log"
	"os"
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

	telegramBot := telegram_api.New(os.Getenv("BOT_TELEGRAM"))
	instagramBot := telegram_api.New(os.Getenv("BOT_INSTAGRAM"))
	trackerBot := telegram_api.New(os.Getenv("BOT_TRACKER"))
	influencerBot := telegram_api.New(os.Getenv("BOT_INFLUENCER"))

	instagramApi := instagram_api.New(os.Getenv("INSTAGRAM_API"))

	app := fiber.New(fiber.Config{
		AppName:           "Go Downloader",
		ErrorHandler:      utils.CustomErrorHandler,
		StreamRequestBody: false,
		Prefork:           false,
		ProxyHeader:       "X-Forwarded-For",
	})

	// Middlewares
	app.Use(etag.New())
	app.Use(earlydata.New())
	app.Use(idempotency.New())
	app.Use(helmet.New())
	app.Use(cors.New())
	app.Use(compress.New(compress.Config{Level: compress.LevelBestCompression}))

	JWTMiddleware := middleware.Authentication

	app.Use(middleware.Attach(&minioClients))
	app.Use(func(ctx *fiber.Ctx) error {
		ctx.Locals("BOT_TELEGRAM", telegramBot)
		ctx.Locals("BOT_INSTAGRAM", instagramBot)
		ctx.Locals("BOT_TRACKER", trackerBot)
		ctx.Locals("BOT_INFLUENCER", influencerBot)
		ctx.Locals("INSTAGRAM_API", instagramApi)

		ctx.Locals("SNITCH_CONFIG", snitchConfiguration)
		return ctx.Next()
	})

	app.Get("/profile/:media/:pk/:userName", controllers.DownloadProfile)

	app.Post("/instant/link", controllers.DownloadFromLinkAndUpload)
	app.Get("/instant/:botName/:fileId", controllers.DownloadFromTelegram)

	app.Post("/direct/:bucketName", JWTMiddleware, controllers.UploadFile)
	app.Get("/direct/*", controllers.DownloadFile)

	log.Printf("Started server on: %s:%s\n", HOST, PORT)
	err = app.Listen(HOST + ":" + PORT)

	if err != nil {
		_ = minioClients.Storage.Close()
		log.Println("Failed to start server")
		panic(err)
	}
}
