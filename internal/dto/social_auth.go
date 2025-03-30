package dto

import "gorm.io/datatypes"

// SocialAuthDTO представляет данные для аутентификации через социальную сеть
type SocialAuthDTO struct {
	Provider       string         `json:"provider" validate:"required"`
	ProviderUserID string         `json:"provider_user_id" validate:"required"`
	Email          string         `json:"email" validate:"required,email"`
	Name           string         `json:"name"`
	ProviderData   datatypes.JSON `json:"provider_data,omitempty"`
}
