package models

import (
	"time"

	"gorm.io/gorm"
)

// User represents the user model
type User struct {
	ID        uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	Name      string         `gorm:"<-:update" json:"name"`
	CreatedAt time.Time      `gorm:"not null;default:NOW()" json:"created_at"`
	UpdatedAt time.Time      `gorm:"not null;default:NOW()" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
