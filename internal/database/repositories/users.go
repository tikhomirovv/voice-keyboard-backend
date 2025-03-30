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

func (r *UsersRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User

	query := r.db.WithContext(ctx).Unscoped()
	if err := query.Where("email = ?", email).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("UsersRepository.FindByEmail: User with email %s not found", email)
		}
		return nil, fmt.Errorf("UsersRepository.FindByEmail: %w", err)
	}
	return &user, nil
}

func (r *UsersRepository) FindByResetPasswordToken(ctx context.Context, token string) (*models.User, error) {
	var user models.User

	if err := r.db.WithContext(ctx).
		Unscoped().
		Where("reset_password_token = ?", token).
		First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("UsersRepository.FindByResetPasswordToken: User with token %s not found", token)
		}
		return nil, fmt.Errorf("UsersRepository.FindByResetPasswordToken: %w", err)
	}
	return &user, nil
}

func (r *UsersRepository) FindByEmailConfirmationCode(ctx context.Context, code string) (*models.User, error) {
	var user models.User

	if err := r.db.WithContext(ctx).
		Unscoped().
		Where("email_confirmation_code = ?", code).
		First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("UsersRepository.FindByEmailConfirmationCode: User with code %s not found", code)
		}
		return nil, fmt.Errorf("UsersRepository.FindByEmailConfirmationCode: %w", err)
	}
	return &user, nil
}

func (r *UsersRepository) FindLast(ctx context.Context) (*models.User, error) {
	var user models.User
	if err := r.db.WithContext(ctx).Last(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("UsersRepository.FindLast: No users found")
		}
		return nil, fmt.Errorf("UsersRepository.FindLast: %w", err)
	}
	return &user, nil
}

func (r *UsersRepository) Exists(ctx context.Context, email string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&models.User{}).
		Unscoped().
		Where("email = ?", email).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("UsersRepository.Exists: %w", err)
	}
	return count > 0, nil
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
