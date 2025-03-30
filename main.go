package main

import (
	"log"
	"os"

	"github.com/urfave/cli/v2"
	"gitlab.com/voice-keyboard/backend-go/commands/application"
)

func main() {
	app := &cli.App{
		Name: "voice-key-backend-go",
		Commands: []*cli.Command{
			application.NewApplicationStartCommand(),
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
