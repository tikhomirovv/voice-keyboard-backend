//go:build wireinject
// +build wireinject

package bootstrap

import (
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

func InitializeContainer(configPath string) (*pkg.Container, error) {
	wire.Build(
		InitializeConfig,
		ProvideLogLevel,
		InitializeLogger,
		InitializeDatabase,
		wire.Struct(new(pkg.Container),
			"Config",
			"Logger",
			"DB",
		),
	)
	return &pkg.Container{}, nil
}
