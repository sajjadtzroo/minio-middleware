package middleware

import (
	jwtware "github.com/gofiber/contrib/jwt"
	"github.com/gofiber/fiber/v2"
	"go-uploader/config"
	"go-uploader/utils"
)

func Authentication(c *fiber.Ctx) error {
	jwtConfig := jwtware.Config{
		SigningKey:   jwtware.SigningKey{Key: []byte(config.GetJwtKey())},
		ErrorHandler: utils.JwtErrorHandler,
	}
	return jwtware.New(jwtConfig)(c)
}
