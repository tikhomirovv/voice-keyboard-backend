package interfaces

import (
	"context"

	"gitlab.com/voice-keyboard/backend-go/internal/dto"
)

// AuthServiceInterface определяет методы для работы с аутентификацией
type AuthServiceInterface interface {
	SignInWithSocial(ctx context.Context, socialAuthDTO *dto.SocialAuthDTO) (*dto.UserTokenDTO, error)
	RefreshTokensPair(ctx context.Context, refresh *dto.RefreshTokensPairDTO) (*dto.UserTokenDTO, error)
}

// YandexOAuthServiceInterface определяет методы для работы с Яндекс OAuth
type YandexOAuthServiceInterface interface {
	GetAuthorizationURL(state string) string
	GetSocialAuthByCode(ctx context.Context, code string) (*dto.SocialAuthDTO, error)
}
