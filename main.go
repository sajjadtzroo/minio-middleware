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
	"go-uploader/utils"
	"log"
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

	// Routes
	app.Get("/", controllers.Health)
	app.Get("/health", controllers.Health)

	api := app.Group("/v1")
	api.Get("/health", controllers.Health)

	api.Get("/get", controllers.DownloadFile)

	log.Printf("Started server on: %s:%s\n", HOST, PORT)
	err = app.Listen(HOST + ":" + PORT)

	if err != nil {
		_ = minioClients.Storage.Close()
		log.Println("Failed to start server")
		panic(err)
	}
}
