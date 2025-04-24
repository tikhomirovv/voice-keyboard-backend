package pkg

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/spf13/viper"
)

type Config struct {
	App        AppConfig        `mapstructure:"app"`
	Cors       CorsConfig       `mapstructure:"cors"`
	Prometheus PrometheusConfig `mapstructure:"prometheus"`
	Database   DatabaseConfig   `mapstructure:"database"`
	Auth       AuthConfig       `mapstructure:"auth"`
	DesktopApp DesktopAppConfig `mapstructure:"desktop_app"`
	WebSocket  WebSocketConfig  `mapstructure:"websocket"`
	Connecte   ConnecteConfig   `mapstructure:"connecte"`
}

type AppConfig struct {
	BaseUrl  string `mapstructure:"base_url"`
	Name     string `mapstructure:"name"`
	Instance string `mapstructure:"instance"`
	Port     uint   `mapstructure:"port"`
	Debug    bool   `mapstructure:"debug"`
	Prefork  bool   `mapstructure:"prefork"`
}

type CorsConfig struct {
	Enabled       bool
	AllowOrigin   string
	AllowMethods  string
	AllowHeaders  string
	ExposeHeaders string
}

type PrometheusConfig struct {
	Port uint
}

type DatabaseConfig struct {
	Host                  string
	Port                  uint16
	Name                  string
	User                  string
	Password              string
	ConnectTimeout        int    `mapstructure:"connect_timeout"`
	MaxConnections        int    `mapstructure:"max_connections"`
	MaxIdleConnections    int    `mapstructure:"max_idle_connections"`
	MaxConnectionLifetime int    `mapstructure:"max_connection_lifetime"`
	MaxConnectionIdleTime int    `mapstructure:"max_connection_idle_time"`
	SslMode               string `mapstructure:"ssl_mode"`
	PreparedStatement     bool   `mapstructure:"prepared_statement"`
}

type AuthConfig struct {
	Secret                string            `mapstructure:"secret"`
	TokenTtl              int               `mapstructure:"token_ttl"`
	ResetPasswordTokenTtl int               `mapstructure:"reset_password_token_ttl"`
	YandexOAuth           YandexOAuthConfig `mapstructure:"yandex"`
}

type YandexOAuthConfig struct {
	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
	RedirectPath string `mapstructure:"redirect_path"`
	AuthURL      string `mapstructure:"auth_url"`
	TokenURL     string `mapstructure:"token_url"`
	UserInfoURL  string `mapstructure:"user_info_url"`
}

type DesktopAppConfig struct {
	Scheme   string `mapstructure:"scheme"`
	AuthPath string `mapstructure:"auth_path"`
}

type WebSocketConfig struct {
	Enabled               bool   `mapstructure:"enabled"`
	Port                  uint   `mapstructure:"port"`
	Path                  string `mapstructure:"path"`
	MaxConnectionsPerUser int    `mapstructure:"max_connections_per_user"`
	MaxSessionDuration    int    `mapstructure:"max_session_duration"`
	IdleTimeout           int    `mapstructure:"idle_timeout"`
	ConnectionTimeout     int    `mapstructure:"connection_timeout"`
}

type ConnecteConfig struct {
	BaseUrl string `mapstructure:"base_url"`
	Token   string `mapstructure:"token"`
}

func (c *Config) GetAppPort() string {
	return fmt.Sprintf(":%d", c.App.Port)
}

func (c *Config) GetPrometheusPort() string {
	return fmt.Sprintf(":%d", c.Prometheus.Port)
}

func (c *Config) GetApplicationName() string {
	return c.App.Name
}

func (c *Config) GetApplicationInstanceName() string {
	return c.App.Instance
}

func (c *Config) GetDatabaseDsnUrl() string {
	dsnUrl, _ := url.Parse(fmt.Sprintf("postgresql://%s:%d/%s", c.Database.Host, c.Database.Port, c.Database.Name))
	dsnUrl.User = url.UserPassword(c.Database.User, c.Database.Password)
	values := dsnUrl.Query()
	values.Set("sslmode", c.Database.SslMode)
	values.Set("connect_timeout", fmt.Sprintf("%d", c.Database.ConnectTimeout))
	values.Set("application_name", c.App.Instance)
	dsnUrl.RawQuery = values.Encode()
	return dsnUrl.String()
}

func (c *Config) BuildSiteUrl(path string, queryParams map[string]string) (string, error) {
	u, err := url.Parse(c.App.BaseUrl)
	if err != nil {
		return "", fmt.Errorf("Config.buildSiteUrl: %w", err)
	}
	u.Path = path
	query := u.Query()
	for key, value := range queryParams {
		query.Set(key, value)
	}
	u.RawQuery = query.Encode()
	return u.String(), nil
}

func (c *Config) GetWebSocketPort() string {
	return fmt.Sprintf(":%d", c.WebSocket.Port)
}

func NewConfig(configPath string) (*Config, error) {
	dir, err := filepath.Abs(filepath.Dir(configPath))
	if err != nil {
		return nil, fmt.Errorf("parse config file path: %w", err)
	}

	configName := filepath.Base(configPath)
	configNameWithoutExt := strings.TrimSuffix(configName, filepath.Ext(configName))

	viper.AddConfigPath(dir)
	viper.SetConfigName(configNameWithoutExt)

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}
	if os.Getenv("HOSTNAME") != "" {
		cfg.App.Instance = os.Getenv("HOSTNAME")
	} else {
		cfg.App.Instance = uuid.New().String()
	}
	return &cfg, nil
}
