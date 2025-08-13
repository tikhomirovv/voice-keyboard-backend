package interfaces

import (
	"encoding/json"
)

// WebSocketHandlerInterface определяет общий интерфейс для обработки WebSocket сообщений
type WebSocketProcessorInterface interface {

	// StartSession инициализирует сессию для обработки аудио
	StartSession(sessionID string, callback func(result string)) error

	// HandleAudioMessage обрабатывает сообщение с аудиоданными
	HandleAudioMessage(sessionID string, data json.RawMessage) error

	// HandleStopMessage обрабатывает сообщение об окончании записи
	HandleStopMessage(sessionID string) (string, error)

	// // CloseSession закрывает сессию и освобождает ресурсы
	CloseSession(sessionID string) error
}
