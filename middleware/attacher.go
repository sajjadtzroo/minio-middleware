package middleware

import (
	"github.com/gofiber/fiber/v2"
	"go-uploader/config"
)

func Attach(minioClients *config.MinIOClients) fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Locals("minio", minioClients)
		return c.Next()
	}
}
