package middleware

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"go-uploader/config"
)

func Attach(minioClients *config.MinIOClients) fiber.Handler {
	if minioClients == nil {
		log.Fatal("MinIO clients cannot be nil")
	}
	return func(c *fiber.Ctx) error {
		c.Locals("minio", minioClients)
		return c.Next()
	}
}
