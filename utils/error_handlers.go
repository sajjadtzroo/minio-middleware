package utils

import (
	"errors"
	"log"

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
	if jsonErr := ctx.Status(code).JSON(models.GenericResponse{
		Result:  false,
		Message: err.Error(),
	}); jsonErr != nil {
		log.Printf("Failed to send error response: %v", jsonErr)
	}

	return nil
}

func JwtErrorHandler(ctx *fiber.Ctx, err error) error {
	code := fiber.StatusUnauthorized

	var e *fiber.Error
	if errors.As(err, &e) {
		code = e.Code
	}

	// Send custom error response
	if jsonErr := ctx.Status(code).JSON(models.GenericResponse{
		Result:  false,
		Message: err.Error(),
	}); jsonErr != nil {
		log.Printf("Failed to send error response: %v", jsonErr)
	}

	return nil
}
