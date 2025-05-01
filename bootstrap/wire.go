//go:build wireinject
// +build wireinject

package bootstrap

import (
	"github.com/go-playground/validator/v10"
	"github.com/google/wire"
	"gitlab.com/voice-keyboard/backend-go/internal/interfaces"
	"gitlab.com/voice-keyboard/backend-go/internal/services"
	"gitlab.com/voice-keyboard/backend-go/internal/services/openai"
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

func InitializeAPIErrorHandler(logger logger.Logger) (*pkg.APIErrorHandler, error) {
	wire.Build(pkg.NewAPIErrorHandler)
	return &pkg.APIErrorHandler{}, nil
}

var ServiceSet = wire.NewSet(
	services.NewYandexOAuthService,
	wire.Bind(new(interfaces.YandexOAuthServiceInterface), new(*services.YandexOAuthService)),
	services.NewAuthService,
	wire.Bind(new(interfaces.AuthServiceInterface), new(*services.AuthService)),
	services.NewAudioService,
	wire.Bind(new(interfaces.AudioServiceInterface), new(*services.AudioService)),
	openai.NewRealtimeTranscriberService,
	wire.Bind(new(interfaces.RealtimeTranscriberServiceInterface), new(*openai.RealtimeTranscriberService)),
	openai.NewOpenAITextGenerationService,
	wire.Bind(new(interfaces.LLMTextGenerationServiceInterface), new(*openai.OpenAITextGenerationService)),
	services.NewReleasesService,
	wire.Bind(new(interfaces.ReleasesServiceInterface), new(*services.ReleasesService)),
)

func InitializeContainer(configPath string) (*pkg.Container, error) {
	wire.Build(
		InitializeConfig,
		ProvideLogLevel,
		InitializeLogger,
		InitializeDatabase,
		InitializeValidator,
		InitializeEmailer,
		InitializeAPIErrorHandler,
		ServiceSet,
		wire.Struct(new(pkg.Container), "*"),
	)
	return &pkg.Container{}, nil
}
