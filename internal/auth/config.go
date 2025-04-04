package auth

type YandexOAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	AuthURL      string
	TokenURL     string
	UserInfoURL  string
}
