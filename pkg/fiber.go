package pkg

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

func NewApp(container *Container) *fiber.App {
	config := container.Config

	errHandler := NewAPIErrorHandler(container.Logger)
	app := fiber.New(fiber.Config{
		IdleTimeout: 5 * time.Second,
		BodyLimit:   100 * 1024 * 1024,
		// Override default error handler
		ErrorHandler: errHandler.HandleError,
		Prefork:      config.App.Prefork,
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
