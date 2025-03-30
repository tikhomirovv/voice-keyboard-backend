package main

import (
	"log"
	"os"

	"github.com/urfave/cli/v2"
	"gitlab.com/voice-keyboard/backend-go/commands/application"
	"gitlab.com/voice-keyboard/backend-go/commands/database"
)

func main() {
	app := &cli.App{
		Name: "voice-key-backend-go",
		Commands: []*cli.Command{
			application.NewApplicationStartCommand(),
			database.NewDatabaseMigrateCommand(),
			database.NewDatabaseRollbackCommand(),
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
