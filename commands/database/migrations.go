package database

import (
	"log"

	"github.com/urfave/cli/v2"
	"gitlab.com/voice-keyboard/backend-go/bootstrap"
	"gitlab.com/voice-keyboard/backend-go/internal/database/migrations"
)

// NewDatabaseMigrateCommand creates a new command for running migrations
func NewDatabaseMigrateCommand() *cli.Command {
	return &cli.Command{
		Name:  "database:migrations:migrate",
		Usage: "Run database migrations",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "config",
				Value: "config.yml",
				Usage: "path to config file",
			},
		},
		Action: func(c *cli.Context) error {
			container, err := bootstrap.InitializeContainer(c.String("config"))
			if err != nil {
				return err
			}

			migrator := migrations.NewMigrator(container.DB)
			if err := migrator.Migrate(); err != nil {
				return err
			}

			log.Println("Migrations completed successfully")
			return nil
		},
	}
}

// NewDatabaseRollbackCommand creates a new command for rolling back migrations
func NewDatabaseRollbackCommand() *cli.Command {
	return &cli.Command{
		Name:  "database:migrations:rollback",
		Usage: "Rollback last database migration",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "config",
				Value: "config.yml",
				Usage: "path to config file",
			},
		},
		Action: func(c *cli.Context) error {
			container, err := bootstrap.InitializeContainer(c.String("config"))
			if err != nil {
				return err
			}

			migrator := migrations.NewMigrator(container.DB)
			if err := migrator.RollbackLast(); err != nil {
				return err
			}

			log.Println("Rollback completed successfully")
			return nil
		},
	}
}
