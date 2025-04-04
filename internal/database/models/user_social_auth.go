package models

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// UserSocialAuth represents social authentication data for a user
type UserSocialAuth struct {
	ID             uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID         uint64         `gorm:"not null" json:"user_id"`
	Provider       string         `gorm:"type:varchar(50);not null" json:"provider"`
	ProviderUserID string         `gorm:"type:varchar(255);not null" json:"provider_user_id"`
	ProviderEmail  string         `gorm:"type:varchar(255)" json:"provider_email"`
	ProviderData   datatypes.JSON `gorm:"type:jsonb" json:"provider_data,omitempty"`
	CreatedAt      time.Time      `gorm:"not null;default:NOW()" json:"created_at"`
	UpdatedAt      time.Time      `gorm:"not null;default:NOW()" json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`

	User User `gorm:"foreignKey:UserID" json:"-"`
}

func (UserSocialAuth) TableName() string {
	return "users_social_auths"
}
