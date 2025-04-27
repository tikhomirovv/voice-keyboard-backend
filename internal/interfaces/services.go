package interfaces

import (
	"context"

	"gitlab.com/voice-keyboard/backend-go/internal/dto"
)

// AuthServiceInterface определяет методы для работы с аутентификацией
type AuthServiceInterface interface {
	SignInWithSocial(ctx context.Context, socialAuthDTO *dto.SocialAuthDTO) (*dto.UserTokenDTO, error)
	RefreshTokensPair(ctx context.Context, refresh *dto.RefreshTokensPairDTO) (*dto.UserTokenDTO, error)
	ValidateToken(tokenString string) (uint64, error)
}

// YandexOAuthServiceInterface определяет методы для работы с Яндекс OAuth
type YandexOAuthServiceInterface interface {
	GetAuthorizationURL(state string) string
	GetSocialAuthByCode(ctx context.Context, code string) (*dto.SocialAuthDTO, error)
}

type AudioServiceInterface interface {
	Create(userID uint64, sessionID string, sampleRate uint32) (string, error)
	WriteData(sessionID string, data []byte) error
	Close(sessionID string) (string, error)
	Remove(userID uint64, sessionID string) error
	GetAudioFilePath(userID uint64, sessionID string, isTemp bool) string
}

// TranscriberServiceInterface определяет методы для транскрибации аудио в текст
type TranscriberServiceInterface interface {
	// Transcribe выполняет транскрибацию аудио в текст
	Transcribe(ctx context.Context, request *dto.TranscriberRequest) (*dto.TranscriberResult, error)
}

// RealtimeTranscriberServiceInterface определяет методы для транскрибации аудио в текст в реальном времени
type RealtimeTranscriberServiceInterface interface {
	// StartSession начинает новую сессию транскрибации в реальном времени
	// Возвращает ID сессии и канал для приема результатов
	StartSession(ctx context.Context, options *dto.RealtimeSessionOptions) (string, <-chan *dto.TranscriberResult, error)

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

// FullTranscriberServiceInterface объединяет возможности обоих интерфейсов транскрибации
// Полезно для сервисов, которые поддерживают оба режима работы
type FullTranscriberServiceInterface interface {
	TranscriberServiceInterface
	RealtimeTranscriberServiceInterface
}

// LLMTextGenerationServiceInterface определяет методы для работы с генерацией текста через OpenAI
type LLMTextGenerationServiceInterface interface {
	// FixText исправляет текст на основе LLM
	FixText(ctx context.Context, text string) (string, error)
}
