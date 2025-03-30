package repositories

import (
	"context"
	"fmt"

	"gitlab.com/voice-keyboard/backend-go/internal/database/models"
	"gorm.io/gorm"
)

type UsersTokensRepository struct {
	db *gorm.DB
}

func (r *UsersTokensRepository) FindByRefreshToken(ctx context.Context, token string) (*models.UserToken, error) {
	var record models.UserToken
	result := r.db.
		Unscoped().
		WithContext(ctx).
		Where("refresh_token = ?", token).
		Find(&record)
	if result.Error != nil {
		return nil, fmt.Errorf("UsersTokensRepository.FindByRefreshToken: %w", result.Error)
	}
	if record.Id == 0 {
		return nil, fmt.Errorf(`UsersTokensRepository.FindByRefreshToken: User token with refresh_token "%s" not found`, token)
	}
	return &record, nil
}

func (r *UsersTokensRepository) Create(ctx context.Context, entity *models.UserToken) error {
	err := r.db.
		WithContext(ctx).
		Create(entity).Error
	if err != nil {
		return fmt.Errorf("UsersTokensRepository.Create: %w", err)
	}
	return nil
}

func (r *UsersTokensRepository) Delete(ctx context.Context, entity *models.UserToken) error {
	result := r.db.WithContext(ctx).Delete(entity)
	if result.Error != nil {
		return fmt.Errorf(`UsersTokensRepository.Delete: User token with id "%d" %w`, entity.Id, result.Error)
	}
	return nil
}

func NewUsersTokensRepository(db *gorm.DB) *UsersTokensRepository {
	return &UsersTokensRepository{db: db}
}
