package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"gitlab.com/voice-keyboard/backend-go/internal/auth"
	"gitlab.com/voice-keyboard/backend-go/internal/dto"
	"gitlab.com/voice-keyboard/backend-go/internal/interfaces"
	"gitlab.com/voice-keyboard/backend-go/pkg"
	"gitlab.com/voice-keyboard/backend-go/pkg/logger"
)

type YandexOAuthService struct {
	config *auth.YandexOAuthConfig
	log    logger.Logger
}

// Убедимся что YandexOAuthService реализует интерфейс
var _ interfaces.YandexOAuthServiceInterface = (*YandexOAuthService)(nil)

func NewYandexOAuthService(cfg *pkg.Config, log logger.Logger) *YandexOAuthService {
	redirectURI, err := cfg.BuildSiteUrl(cfg.YandexOAuth.RedirectPath, nil)
	if err != nil {
		log.Error("YandexOAuthService.NewYandexOAuthService", "Failed to build redirect URI", "error", err)
	}

	// Добавим логирование для отладки
	log.Info("YandexOAuthService.NewYandexOAuthService",
		"clientID", cfg.YandexOAuth.ClientID,
		"redirectURI", redirectURI,
		"authURL", cfg.YandexOAuth.AuthURL,
		"tokenURL", cfg.YandexOAuth.TokenURL,
		"userInfoURL", cfg.YandexOAuth.UserInfoURL,
	)

	return &YandexOAuthService{
		config: &auth.YandexOAuthConfig{
			ClientID:     cfg.YandexOAuth.ClientID,
			ClientSecret: cfg.YandexOAuth.ClientSecret,
			RedirectURI:  redirectURI,
			AuthURL:      cfg.YandexOAuth.AuthURL,
			TokenURL:     cfg.YandexOAuth.TokenURL,
			UserInfoURL:  cfg.YandexOAuth.UserInfoURL,
		},
		log: log,
	}
}

// GetAuthorizationURL генерирует URL для авторизации пользователя
func (s *YandexOAuthService) GetAuthorizationURL(state string) string {
	params := url.Values{}
	params.Add("response_type", "code")
	params.Add("client_id", s.config.ClientID)
	params.Add("redirect_uri", s.config.RedirectURI)
	params.Add("state", state)
	// Добавляем scope для получения информации о пользователе
	params.Add("scope", "login:info login:email")

	return fmt.Sprintf("%s?%s", s.config.AuthURL, params.Encode())
}

// ExchangeCodeForToken обменивает код авторизации на токен
func (s *YandexOAuthService) exchangeCodeForToken(ctx context.Context, code string) (*dto.YandexTokenResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("client_id", s.config.ClientID)
	data.Set("client_secret", s.config.ClientSecret)
	data.Set("redirect_uri", s.config.RedirectURI)

	req, err := http.NewRequestWithContext(ctx, "POST", s.config.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("YandexOAuthService.ExchangeCodeForToken: create token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("YandexOAuthService.ExchangeCodeForToken: token request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errorResp struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&errorResp); err != nil {
			return nil, fmt.Errorf("YandexOAuthService.ExchangeCodeForToken: failed to decode error response: %w", err)
		}
		return nil, fmt.Errorf("YandexOAuthService.ExchangeCodeForToken: oauth error: %s - %s", errorResp.Error, errorResp.ErrorDescription)
	}

	var tokenResp dto.YandexTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("YandexOAuthService.ExchangeCodeForToken: decode token response: %w", err)
	}

	return &tokenResp, nil
}

// GetUserInfo получает информацию о пользователе из API Яндекса
func (s *YandexOAuthService) getUserInfo(ctx context.Context, accessToken string) (*dto.YandexUserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", s.config.UserInfoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("YandexOAuthService.GetUserInfo: create user info request: %w", err)
	}

	req.Header.Set("Authorization", "OAuth "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("YandexOAuthService.GetUserInfo: user info request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("YandexOAuthService.GetUserInfo: user info request failed with status %d", resp.StatusCode)
	}

	var userInfo dto.YandexUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, fmt.Errorf("YandexOAuthService.GetUserInfo: decode user info response: %w", err)
	}

	return &userInfo, nil
}

func (s *YandexOAuthService) GetSocialAuthByCode(ctx context.Context, code string) (*dto.SocialAuthDTO, error) {
	// Обмениваем код на токен
	token, err := s.exchangeCodeForToken(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("YandexOAuthService.GetSocialAuthByCode: exchange code for token: %w", err)
	}
	s.log.Debug("YandexOAuthService.GetSocialAuthByCode", "token", token)

	// Получаем информацию о пользователе из Яндекса
	userInfo, err := s.getUserInfo(ctx, token.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("YandexOAuthService.GetSocialAuthByCode: get user info: %w", err)
	}
	s.log.Debug("YandexOAuthService.GetSocialAuthByCode", "userInfo", userInfo)

	// Создаем DTO для авторизации
	providerData, err := json.Marshal(map[string]any{
		"name":      userInfo.Name,
		"login":     userInfo.Login,
		"avatar":    userInfo.AvatarURL,
		"real_name": userInfo.RealName,
	})
	if err != nil {
		return nil, fmt.Errorf("YandexOAuthService.GetSocialAuthByCode: marshal provider data: %w", err)
	}

	name := userInfo.Name
	if name == "" {
		name = userInfo.RealName
	}
	if name == "" {
		name = userInfo.Login
	}
	socialAuthDTO := &dto.SocialAuthDTO{
		Provider:       "yandex",
		ProviderUserID: userInfo.ID,
		Name:           name,
		Email:          userInfo.Email,
		ProviderData:   providerData,
	}

	return socialAuthDTO, nil
}
