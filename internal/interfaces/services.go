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
