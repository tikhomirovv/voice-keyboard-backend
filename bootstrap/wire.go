//go:build wireinject
// +build wireinject

package bootstrap

import (
	"github.com/go-playground/validator/v10"
	"github.com/google/wire"
	"gitlab.com/voice-keyboard/backend-go/pkg"
	"gitlab.com/voice-keyboard/backend-go/pkg/logger"
	"gorm.io/gorm"
)

func ProvideLogLevel(cfg *pkg.Config) logger.LogLevel {
	if cfg.App.Debug {
		return "debug"
	}
	return "info"
}

func InitializeConfig(configPath string) (*pkg.Config, error) {
	wire.Build(pkg.NewConfig)
	return &pkg.Config{}, nil
}

func InitializeLogger(logLevel logger.LogLevel) logger.Logger {
	wire.Build(logger.NewLogger)
	return &logger.ZLogger{}
}

func InitializeDatabase(cfg *pkg.Config) (*gorm.DB, error) {
	wire.Build(pkg.NewDatabase)
	return &gorm.DB{}, nil
}

func InitializeValidator() (*validator.Validate, error) {
	wire.Build(pkg.NewValidator)
	return &validator.Validate{}, nil
}

func InitializeEmailer(cfg *pkg.Config, logger logger.Logger) (*pkg.Emailer, error) {
	wire.Build(pkg.NewEmailer)
	return &pkg.Emailer{}, nil
}

func InitializeContainer(configPath string) (*pkg.Container, error) {
	wire.Build(
		InitializeConfig,
		ProvideLogLevel,
		InitializeLogger,
		InitializeDatabase,
		InitializeValidator,
		InitializeEmailer,
		wire.Struct(new(pkg.Container),
			"Config",
			"Logger",
			"DB",
			"Validator",
			"Emailer",
		),
	)
	return &pkg.Container{}, nil
}
