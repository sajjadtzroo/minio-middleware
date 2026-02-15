package middleware

import (
	"go-uploader/config"
	"go-uploader/utils"

	jwtware "github.com/gofiber/contrib/jwt"
	"github.com/gofiber/fiber/v2"
)

// Authentication is the JWT middleware handler, initialized once at startup
var Authentication fiber.Handler

func init() {
	key := config.GetJwtKey()
	Authentication = jwtware.New(jwtware.Config{
		SigningKey:  jwtware.SigningKey{JWTAlg: "HS256", Key: []byte(key)},
		ErrorHandler: utils.JwtErrorHandler,
	})
}
