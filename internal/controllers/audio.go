package controllers

import (
	"fmt"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"gitlab.com/voice-keyboard/backend-go/internal/interfaces"
	"gitlab.com/voice-keyboard/backend-go/pkg"
	"gitlab.com/voice-keyboard/backend-go/pkg/logger"
)

// AudioController обрабатывает запросы связанные с аудиофайлами
type AudioController struct {
	log          logger.Logger
	audioService interfaces.AudioServiceInterface
}

// GetAudioFileWavAction отдает WAV-файл по userID и sessionID
func (ac *AudioController) GetAudioFileWavAction(c *fiber.Ctx) error {
	// Получаем параметры из URL
	userIDStr := c.Params("userID")
	sessionID := c.Params("sessionID")

	// Проверяем наличие параметров
	if userIDStr == "" || sessionID == "" {
		return &fiber.Error{
			Code:    fiber.StatusBadRequest,
			Message: "userID and sessionID are required",
		}
	}

	// Конвертируем userID в uint64
	userID, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		ac.log.Error(fmt.Sprintf("Failed to parse userID: %v", err))
		return &fiber.Error{
			Code:    fiber.StatusBadRequest,
			Message: "Invalid userID format",
		}
	}

	// Получаем путь к файлу через AudioService
	filePath := ac.audioService.GetAudioFilePath(userID, sessionID, false) // false = не временный файл (WAV)

	// Устанавливаем правильный Content-Type
	c.Set("Content-Type", "audio/wav")

	// Отправляем файл клиенту
	return c.SendFile(filePath)
}

// RegisterAudioController регистрирует маршруты для AudioController
func RegisterAudioController(router fiber.Router, container *pkg.Container) {
	ctrl := NewAudioController(container)
	router.Get("/:userID/:sessionID.wav", ctrl.GetAudioFileWavAction)
}

// NewAudioController создает новый экземпляр AudioController
func NewAudioController(cnt *pkg.Container) *AudioController {
	return &AudioController{
		log:          cnt.Logger,
		audioService: cnt.AudioService,
	}
}
