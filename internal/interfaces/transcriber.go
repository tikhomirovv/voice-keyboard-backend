package interfaces

import (
	"context"

	"gitlab.com/voice-keyboard/backend-go/internal/dto"
)

// TranscriberServiceInterface определяет методы для транскрибации аудио в текст
type TranscriberServiceInterface interface {
	// Transcribe выполняет транскрибацию аудио в текст
	Transcribe(ctx context.Context, request *dto.TranscriberRequest) (*dto.TranscriberResult, error)
}

// RealtimeTranscriberServiceInterface определяет методы для транскрибации аудио в текст в реальном времени
type RealtimeTranscriberServiceInterface interface {
	// StartSession начинает новую сессию транскрибации в реальном времени
	// Возвращает ID сессии и канал для приема результатов
	StartSession(ctx context.Context, options *RealtimeSessionOptions) (string, <-chan *dto.TranscriberResult, error)

	// AppendAudio добавляет аудиоданные в текущую сессию
	// Данные должны быть в формате base64
	AppendAudio(ctx context.Context, sessionID string, audioData string) error

	// CompleteSession завершает сессию транскрибации и возвращает финальный результат
	CompleteSession(ctx context.Context, sessionID string) (*dto.TranscriberResult, error)

	// CloseSession закрывает сессию транскрибации и освобождает ресурсы
	CloseSession(ctx context.Context, sessionID string) error

	// Close освобождает ресурсы сервиса и закрывает все активные сессии
	Close() error
}

// RealtimeSessionOptions представляет параметры для создания сессии транскрибации в реальном времени
type RealtimeSessionOptions struct {
	// SessionID идентификатор сессии транскрибации
	SessionID string `json:"session_id" validate:"required"`

	// Format формат аудиоданных (например, "pcm16")
	Format string `json:"format" validate:"required"`

	// Language язык аудио (например, "ru"), если известен
	Language string `json:"language,omitempty"`

	// Prompt подсказка для модели транскрипции, может помочь с точностью
	Prompt string `json:"prompt,omitempty"`
}
