package dto

// YandexTokenResponse представляет ответ с токеном от Яндекса
type YandexTokenResponse struct {
	TokenType    string `json:"token_type"`
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

// YandexUserInfo представляет информацию о пользователе от Яндекса
type YandexUserInfo struct {
	ID        string `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	Email     string `json:"default_email"`
	RealName  string `json:"real_name"`
	AvatarURL string `json:"default_avatar_id"`
}
