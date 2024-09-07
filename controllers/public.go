package controllers

import (
	"github.com/gofiber/fiber/v2"
	"go-uploader/models"
)

func Health(ctx *fiber.Ctx) error {
	return ctx.Status(200).JSON(models.HealthCheckResponse{
		Result:  true,
		Message: "server is up and running",
		Ip:      ctx.IP(),
	})
}
