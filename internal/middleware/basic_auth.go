package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/basicauth"
	"gitlab.com/voice-keyboard/backend-go/pkg"
	"gitlab.com/voice-keyboard/backend-go/pkg/logger"
)

// NewBasicAuthMiddleware создает middleware для Basic Authentication
func NewBasicAuthMiddleware(config *pkg.Config, logger logger.Logger) fiber.Handler {
	logger.Info("Basic auth middleware initialized",
		"username_set", config.BasicAuth.Username != "",
		"password_set", config.BasicAuth.Password != "")

	// Проверяем, настроена ли аутентификация
	if config.BasicAuth.Username == "" || config.BasicAuth.Password == "" {
		logger.Warn("Basic auth credentials not properly configured")
		// Если учетные данные не настроены, все равно требуем аутентификацию
		// для безопасности, но с пустым списком пользователей никто не сможет войти
		return basicauth.New(basicauth.Config{
			Users: map[string]string{},
			Realm: "Restricted Area",
			Unauthorized: func(c *fiber.Ctx) error {
				logger.Info("Unauthorized access attempt - no valid credentials configured")
				// Устанавливаем заголовки для гарантированного запроса аутентификации
				c.Set("WWW-Authenticate", `Basic realm="Restricted Area"`)
				// Отключаем кэширование
				c.Set("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate")
				c.Set("Pragma", "no-cache")
				c.Set("Expires", "0")
				return c.Status(fiber.StatusUnauthorized).SendString("Authentication required but not properly configured")
			},
		})
	}

	// Создаем middleware с Basic Authentication
	return basicauth.New(basicauth.Config{
		Users: map[string]string{
			config.BasicAuth.Username: config.BasicAuth.Password,
		},
		Realm: "Monitor Access",
		Unauthorized: func(c *fiber.Ctx) error {
			logger.Info("Unauthorized access attempt")
			// Устанавливаем заголовки для гарантированного запроса аутентификации
			c.Set("WWW-Authenticate", `Basic realm="Monitor Access"`)
			// Отключаем кэширование
			c.Set("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate")
			c.Set("Pragma", "no-cache")
			c.Set("Expires", "0")
			return c.Status(fiber.StatusUnauthorized).SendString("Unauthorized")
		},
		ContextUsername: "username",
	})
}
