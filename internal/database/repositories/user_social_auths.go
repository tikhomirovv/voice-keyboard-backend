package repositories

import (
	"context"
	"fmt"

	"gitlab.com/voice-keyboard/backend-go/internal/database/models"
	"gorm.io/gorm"
)

type UserSocialAuthsRepository struct {
	db *gorm.DB
}

func (r *UserSocialAuthsRepository) FindByProviderAndID(ctx context.Context, provider, providerUserID string) (*models.UserSocialAuth, error) {
	var auth models.UserSocialAuth

	if err := r.db.WithContext(ctx).
		Where("provider = ? AND provider_user_id = ?", provider, providerUserID).
		First(&auth).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("UserSocialAuthsRepository.FindByProviderAndID: Auth with provider %s and ID %s not found", provider, providerUserID)
		}
		return nil, fmt.Errorf("UserSocialAuthsRepository.FindByProviderAndID: %w", err)
	}
	return &auth, nil
}

func (r *UserSocialAuthsRepository) FindByUserID(ctx context.Context, userID int) ([]*models.UserSocialAuth, error) {
	var auths []*models.UserSocialAuth

	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Find(&auths).Error; err != nil {
		return nil, fmt.Errorf("UserSocialAuthsRepository.FindByUserID: %w", err)
	}
	return auths, nil
}

func (r *UserSocialAuthsRepository) Create(ctx context.Context, auth *models.UserSocialAuth) error {
	if err := r.db.WithContext(ctx).Create(auth).Error; err != nil {
		return fmt.Errorf("UserSocialAuthsRepository.Create: %w", err)
	}
	return nil
}

func (r *UserSocialAuthsRepository) Delete(ctx context.Context, auth *models.UserSocialAuth) error {
	if err := r.db.WithContext(ctx).Delete(auth).Error; err != nil {
		return fmt.Errorf("UserSocialAuthsRepository.Delete: %w", err)
	}
	return nil
}

func (r *UserSocialAuthsRepository) Update(ctx context.Context, auth *models.UserSocialAuth) error {
	if err := r.db.WithContext(ctx).Save(auth).Error; err != nil {
		return fmt.Errorf("UserSocialAuthsRepository.Update: %w", err)
	}
	return nil
}

func NewUserSocialAuthsRepository(db *gorm.DB) *UserSocialAuthsRepository {
	return &UserSocialAuthsRepository{db: db}
}
