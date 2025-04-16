package routers

import (
	"github.com/gofiber/fiber/v2"
	"gitlab.com/voice-keyboard/backend-go/pkg"
)

// NewAudioRouter создает и возвращает роутер для аудиофайлов
func NewAudioRouter(router fiber.Router, cnt *pkg.Container) fiber.Router {
	return router.Group("/audio")
}
