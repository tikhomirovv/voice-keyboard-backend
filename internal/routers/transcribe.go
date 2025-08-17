package routers

import (
	"github.com/gofiber/fiber/v2"
	"gitlab.com/voice-keyboard/backend-go/pkg"
)

// NewWebSocketRouter создает и возвращает роутер для WebSocket
func NewWebSocketRouter(router fiber.Router, cnt *pkg.Container) fiber.Router {
	return router.Group("/transcribe")
}
