package models

import (
	"time"
)

type UserToken struct {
	Id           uint64    `json:"id" gorm:"primaryKey"`
	UserId       uint64    `json:"user_id"`
	RefreshToken string    `json:"refresh_token"`
	CreatedAt    time.Time `json:"created_at"`
	ExpiresAt    time.Time `json:"expires_at"`
}
