package services

import (
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"gitlab.com/voice-keyboard/backend-go/internal/auth"
	"gitlab.com/voice-keyboard/backend-go/internal/database/models"
	"gitlab.com/voice-keyboard/backend-go/internal/database/repositories"
	"gitlab.com/voice-keyboard/backend-go/internal/dto"
	"gitlab.com/voice-keyboard/backend-go/internal/interfaces"
	"gitlab.com/voice-keyboard/backend-go/pkg"
	"gitlab.com/voice-keyboard/backend-go/pkg/logger"
	"gorm.io/gorm"
)

type AuthService struct {
	db          *gorm.DB
	cfg         *pkg.Config
	log         logger.Logger
	em          *pkg.Emailer
	yandexOAuth interfaces.YandexOAuthServiceInterface
}

// Убедимся что AuthService реализует интерфейс
var _ interfaces.AuthServiceInterface = (*AuthService)(nil)

func (as *AuthService) RefreshTokensPair(ctx context.Context, refresh *dto.RefreshTokensPairDTO) (*dto.UserTokenDTO, error) {
	usersTokensRep := repositories.NewUsersTokensRepository(as.db)
	userToken, err := usersTokensRep.FindByRefreshToken(ctx, refresh.RefreshToken)
	if err != nil {
		return nil, fmt.Errorf("AuthService.RefreshTokensPair: %w", err)
	}

	var token *dto.UserTokenDTO
	err = as.db.Transaction(func(tx *gorm.DB) error {
		// delete current session
		errTx := usersTokensRep.Delete(ctx, userToken)
		if errTx != nil {
			return errTx
		}
		// generate new session
		userToken, errTx := as.generateAndSaveNewUserToken(ctx, tx, userToken.UserId)
		if errTx != nil {
			return errTx
		}
		token, errTx = as.generateUserTokensPair(userToken)
		if errTx != nil {
			return errTx
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("AuthService.RefreshTokensPair: %w", err)
	}
	return token, nil
}

func (as *AuthService) generateUserTokensPair(userToken *models.UserToken) (*dto.UserTokenDTO, error) {
	// Create the Claims
	claims := &auth.JwtClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: &jwt.NumericDate{
				Time: userToken.ExpiresAt,
			},
			IssuedAt: &jwt.NumericDate{
				Time: userToken.CreatedAt,
			},
		},
		User: auth.User{
			ID: userToken.UserId,
		},
	}

	// Create token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	secret := as.cfg.Auth.Secret
	t, err := token.SignedString([]byte(secret))
	if err != nil {
		return nil, err
	}
	res := &dto.UserTokenDTO{
		AccessToken:  t,
		RefreshToken: userToken.RefreshToken,
		Type:         "Bearer",
	}
	return res, nil
}

// generateAndSaveNewUserToken генерирует новый токен и сохраняет его в БД
func (as *AuthService) generateAndSaveNewUserToken(ctx context.Context, db *gorm.DB, userId uint64) (*models.UserToken, error) {
	userToken := as.generateNewUserToken(userId)
	usersTokensRep := repositories.NewUsersTokensRepository(db)
	err := usersTokensRep.Create(ctx, userToken)
	if err != nil {
		return nil, fmt.Errorf("AuthService.SignInUser: %w", err)
	}

	return userToken, nil
}

func (as *AuthService) generateNewUserToken(userId uint64) *models.UserToken {
	createdAt := time.Now().UTC()
	expiresAt := createdAt.Add(time.Duration(as.cfg.Auth.TokenTtl) * time.Second)
	return &models.UserToken{
		UserId:       userId,
		RefreshToken: as.generateNewRefreshTokenString(),
		CreatedAt:    createdAt,
		ExpiresAt:    expiresAt,
	}
}

func (as *AuthService) generateNewRefreshTokenString() string {
	secret := as.cfg.Auth.Secret
	mac := hmac.New(sha512.New, []byte(secret))
	mac.Write([]byte(uuid.New().String()))
	bsm := mac.Sum(nil)
	return base64.StdEncoding.EncodeToString(bsm)
}

// SignInWithSocial аутентифицирует пользователя через социальную сеть
func (as *AuthService) SignInWithSocial(ctx context.Context, socialData *dto.SocialAuthDTO) (*dto.UserTokenDTO, error) {
	socialAuthsRepo := repositories.NewUserSocialAuthsRepository(as.db)

	// Проверяем существующую привязку к соцсети по provider + provider_user_id
	socialAuth, err := socialAuthsRepo.FindByProviderAndID(ctx, socialData.Provider, socialData.ProviderUserID)
	if err == nil {
		// Нашли существующую привязку - авторизуем
		userToken, err := as.generateAndSaveNewUserToken(ctx, as.db, socialAuth.UserID)
		if err != nil {
			return nil, fmt.Errorf("SignInWithSocial generate token: %w", err)
		}
		return as.generateUserTokensPair(userToken)
	}

	// Создаем нового пользователя и привязку к соцсети
	var userToken *models.UserToken
	err = as.db.Transaction(func(tx *gorm.DB) error {
		// Создаем нового пользователя только с именем
		user := &models.User{
			Name: socialData.Name,
		}
		if err := tx.Create(user).Error; err != nil {
			return fmt.Errorf("create user: %w", err)
		}

		// Создаем привязку к соцсети
		socialAuth = &models.UserSocialAuth{
			UserID:         user.ID,
			Provider:       socialData.Provider,
			ProviderUserID: socialData.ProviderUserID,
			ProviderEmail:  socialData.Email, // Сохраняем email, но не используем для идентификации
			ProviderData:   socialData.ProviderData,
		}
		if err := tx.Create(socialAuth).Error; err != nil {
			return fmt.Errorf("create social auth: %w", err)
		}

		userToken, err = as.generateAndSaveNewUserToken(ctx, tx, user.ID)
		if err != nil {
			return fmt.Errorf("generate token: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("SignInWithSocial transaction: %w", err)
	}

	return as.generateUserTokensPair(userToken)
}

// LinkSocialAccount привязывает дополнительную соц. сеть к существующему аккаунту
func (as *AuthService) LinkSocialAccount(ctx context.Context, userID uint64, socialData *dto.SocialAuthDTO) error {
	socialAuthsRepo := repositories.NewUserSocialAuthsRepository(as.db)

	// Проверяем, не привязан ли уже этот аккаунт соц. сети к другому пользователю
	existing, err := socialAuthsRepo.FindByProviderAndID(ctx, socialData.Provider, socialData.ProviderUserID)
	if err == nil && existing.UserID != userID {
		return fmt.Errorf("social account already linked to another user")
	}

	// Создаем новую привязку
	socialAuth := &models.UserSocialAuth{
		UserID:         userID,
		Provider:       socialData.Provider,
		ProviderUserID: socialData.ProviderUserID,
		ProviderEmail:  socialData.Email,
		ProviderData:   socialData.ProviderData,
	}

	if err := socialAuthsRepo.Create(ctx, socialAuth); err != nil {
		return fmt.Errorf("link social account: %w", err)
	}

	return nil
}

func NewAuthService(
	db *gorm.DB,
	cfg *pkg.Config,
	log logger.Logger,
	em *pkg.Emailer,
	yandexOAuth interfaces.YandexOAuthServiceInterface,
) *AuthService {
	return &AuthService{
		db:          db,
		cfg:         cfg,
		log:         log,
		em:          em,
		yandexOAuth: yandexOAuth,
	}
}
