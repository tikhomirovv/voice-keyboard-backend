package pkg

import (
	"github.com/go-playground/validator/v10"
	"gitlab.com/voice-keyboard/backend-go/internal/interfaces"
	"gitlab.com/voice-keyboard/backend-go/pkg/logger"
	"gorm.io/gorm"
)

type Container struct {
	Config                     *Config
	Logger                     logger.Logger
	DB                         *gorm.DB
	Emailer                    *Emailer
	Validator                  *validator.Validate
	ErrorHandler               *APIErrorHandler
	AuthService                interfaces.AuthServiceInterface
	YandexOAuthService         interfaces.YandexOAuthServiceInterface
	AudioService               interfaces.AudioServiceInterface
	RealtimeTranscriberService interfaces.RealtimeTranscriberServiceInterface
}
