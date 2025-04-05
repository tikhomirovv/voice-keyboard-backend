package pkg

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/template/html/v2"
)

func NewApp(container *Container) *fiber.App {
	config := container.Config

	// Инициализация движка шаблонов
	engine := html.New("./internal/views", ".html")

	// Включаем отладку для просмотра ошибок парсинга шаблонов
	engine.Debug(config.App.Debug)
	// Включаем перезагрузку шаблонов в режиме разработки
	engine.Reload(config.App.Debug)

	errHandler := NewAPIErrorHandler(container.Logger)
	app := fiber.New(fiber.Config{
		IdleTimeout: 5 * time.Second,
		BodyLimit:   100 * 1024 * 1024,
		// Override default error handler
		ErrorHandler: errHandler.HandleError,
		Prefork:      config.App.Prefork,
		// Подключаем движок шаблонов
		Views: engine,
	})
	app.Use(recover.New())
	if config.Cors.Enabled {
		app.Use(cors.New(cors.Config{
			AllowHeaders:  config.Cors.AllowHeaders,
			AllowMethods:  config.Cors.AllowMethods,
			AllowOrigins:  config.Cors.AllowOrigin,
			ExposeHeaders: config.Cors.ExposeHeaders,
		}))
	}
	return app
}
