package repositories

import (
	"context"
	"fmt"

	"gitlab.com/voice-keyboard/backend-go/internal/database/models"
	"gorm.io/gorm"
)

type UsersRepository struct {
	db *gorm.DB
}

func (r *UsersRepository) FindAll(ctx context.Context) ([]*models.User, error) {
	var users []*models.User
	if err := r.db.WithContext(ctx).Find(&users).Error; err != nil {
		return nil, fmt.Errorf("UsersRepository.FindAll: %w", err)
	}
	return users, nil
}

func (r *UsersRepository) FindById(ctx context.Context, id uint64) (*models.User, error) {
	var user models.User

	query := r.db.WithContext(ctx).Unscoped()
	if err := query.Where("id = ?", id).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("UsersRepository.FindById: User with id %d not found", id)
		}
		return nil, fmt.Errorf("UsersRepository.FindById: %w", err)
	}
	return &user, nil
}

func (r *UsersRepository) Create(ctx context.Context, user *models.User) error {
	if err := r.db.WithContext(ctx).Create(user).Error; err != nil {
		return fmt.Errorf("UsersRepository.Create: %w", err)
	}
	return nil
}

func (r *UsersRepository) Update(ctx context.Context, user *models.User) error {
	if err := r.db.WithContext(ctx).Save(user).Error; err != nil {
		return fmt.Errorf("UsersRepository.Update: %w", err)
	}
	return nil
}

func NewUsersRepository(db *gorm.DB) *UsersRepository {
	return &UsersRepository{db: db}
}
