package interfaces

import (
	"context"
	"os"

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
	CreateAudioFile(userID, sessionID string) (string, *os.File, error)
	WriteAudioData(file *os.File, data []byte) error
	CloseAudioFile(file *os.File, sampleRate uint32) (string, error)
	RemoveAudioFile(filePath string) error
}
