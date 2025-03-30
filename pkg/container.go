package pkg

import (
	"gitlab.com/voice-keyboard/backend-go/pkg/logger"
	"gorm.io/gorm"
)

type Container struct {
	Config *Config
	Logger logger.Logger
	DB     *gorm.DB
}
