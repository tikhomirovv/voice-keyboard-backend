package routers

import (
	"strings"

	jwtware "github.com/gofiber/contrib/jwt"
	"github.com/gofiber/fiber/v2"
	"gitlab.com/voice-keyboard/backend-go/pkg"
)

func NewAuthRouter(router fiber.Router, cnt *pkg.Container) fiber.Router {
	secret := cnt.Config.Auth.Secret
	return router.Group("/auth", jwtware.New(jwtware.Config{
		SigningKey:   jwtware.SigningKey{Key: []byte(secret)},
		ErrorHandler: cnt.ErrorHandler.HandleAuthError,
		Filter: func(c *fiber.Ctx) bool {
			return !strings.Contains(c.Path(), "auth/signout")
		},
	}))
}
