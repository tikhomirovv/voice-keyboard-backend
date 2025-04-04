package application

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/urfave/cli/v2"
	"gitlab.com/voice-keyboard/backend-go/bootstrap"
	"gitlab.com/voice-keyboard/backend-go/healthcheck"
	"gitlab.com/voice-keyboard/backend-go/internal/controllers"
	"gitlab.com/voice-keyboard/backend-go/internal/routers"
	"gitlab.com/voice-keyboard/backend-go/pkg"
)

func NewApplicationStartCommand() *cli.Command {
	return &cli.Command{
		Name:  "app:start",
		Usage: "Application start command",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "config",
				Value: "config.yml",
				Usage: "Yml config file path",
			},
		},
		Action: func(cCtx *cli.Context) error {
			configPath := cCtx.String("config")
			container, err := bootstrap.InitializeContainer(configPath)
			if err != nil {
				return err
			}
			config := container.Config
			logger := container.Logger

			app := pkg.NewApp(container)
			// public router
			publicRouter := app.Group("/")
			controllers.RegisterPublicController(publicRouter, container)

			// api router
			mainRouter := app.Group("/api")
			healthcheck.RegisterHealthCheckController(mainRouter, container)

			// api v1 router
			apiV1Router := mainRouter.Group("/v1")
			authRouter := routers.NewAuthRouter(apiV1Router, container)
			controllers.RegisterAuthController(authRouter, container)

			logger.Info(fmt.Sprintf(`%s instance "%s" started`, config.App.Name, config.App.Instance))
			// Listen web app
			go func() {
				if err := app.Listen(config.GetAppPort()); err != nil {
					log.Fatal(err)
				}
			}()

			c := make(chan os.Signal, 1)                                    // Create channel to signify a signal being sent
			signal.Notify(c, os.Interrupt, syscall.SIGINT, syscall.SIGTERM) // When an interrupt or termination signal is sent, notify the channel
			<-c                                                             // This blocks the main thread until an interrupt is received
			logger.Info(fmt.Sprintf(`%s instance "%s" gracefully shutting down...`, config.App.Name, config.App.Instance))
			_ = app.Shutdown()
			logger.Info(fmt.Sprintf(`%s instance "%s" was successful shutdown.`, config.App.Name, config.App.Instance))

			return nil
		},
	}
}
