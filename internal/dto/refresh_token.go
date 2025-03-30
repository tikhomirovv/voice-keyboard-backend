package dto

type RefreshTokensPairDTO struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}
