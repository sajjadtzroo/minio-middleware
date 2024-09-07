package utils

import (
	"errors"
	"github.com/gofiber/fiber/v2"
	"go-uploader/models"
)

func CustomErrorHandler(ctx *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError

	var e *fiber.Error
	if errors.As(err, &e) {
		code = e.Code
	}

	// Send custom error response
	err = ctx.Status(code).JSON(models.GenericResponse{
		Result:  false,
		Message: err.Error(),
	})

	return nil
}
