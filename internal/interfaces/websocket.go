package interfaces

import (
	"encoding/json"
)

type ProcessorSessionOptions struct {
	UserID     uint64
	Format     string
	SampleRate uint32
	Callback   func(text string)
}

// WebSocketHandlerInterface определяет общий интерфейс для обработки WebSocket сообщений
type WebSocketProcessorInterface interface {

	// StartSession инициализирует сессию для обработки аудио
	StartSession(sessionID string, options *ProcessorSessionOptions) error

	// HandleAudioMessage обрабатывает сообщение с аудиоданными
	HandleAudioMessage(sessionID string, data json.RawMessage) error

	// HandleStopMessage обрабатывает сообщение об окончании записи
	HandleStopMessage(sessionID string) (string, error)

	// // CloseSession закрывает сессию и освобождает ресурсы
	CloseSession(sessionID string) error
}
