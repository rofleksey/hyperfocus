package config

import (
	"hyperfocus/app/api"
	"os"

	"github.com/caarlos0/env/v11"
	"github.com/go-playground/validator/v10"
	"github.com/samber/oops"
	"gopkg.in/yaml.v3"
)

type Config struct {
	// Service name for telemetry and logs
	ServiceName string `yaml:"service_name" env:"SERVICE_NAME" example:"hyperfocus" validate:"required"`
	// Base url of the service, needed for some redirects
	BaseURL    string     `yaml:"base_url" env:"BASE_URL" example:"https://hyperfocus.rofleksey.ru" validate:"required"`
	Sentry     Sentry     `yaml:"sentry" envPrefix:"SENTRY_"`
	Log        Log        `yaml:"log" envPrefix:"LOG_"`
	Telemetry  Telemetry  `yaml:"telemetry" envPrefix:"TELEMETRY_"`
	DB         DB         `yaml:"db" envPrefix:"DB_"`
	Auth       Auth       `yaml:"auth" envPrefix:"AUTH_"`
	Admin      Admin      `yaml:"admin" envPrefix:"ADMIN_"`
	Twitch     Twitch     `yaml:"twitch" envPrefix:"TWITCH_"`
	Paddle     Paddle     `yaml:"paddle" envPrefix:"PADDLE_"`
	Processing Processing `yaml:"processing" envPrefix:"PROCESSING_"`
	Server     Server     `yaml:"server" envPrefix:"SERVER_"`
}

type Sentry struct {
	DSN string `yaml:"dsn" env:"DSN" example:"https://a1b2c3d4e5f6g7h8a1b2c3d4e5f6g7h8@o123456.ingest.sentry.io/1234567"`
}

type Log struct {
	// Telegram logging config
	Telegram TelegramLog `yaml:"telegram" envPrefix:"TELEGRAM_"`
}

type TelegramLog struct {
	// Chat bot token, obtain it via BotFather
	Token string `yaml:"token" env:"TOKEN" example:"1234567890:ABCdefGHIjklMNopQRstUVwxyZ-123456789"`
	// Chat ID to send messages to
	ChatID string `yaml:"chat_id" env:"CHAT_ID" example:"1001234567890"`
}

type Telemetry struct {
	// Whether to enable opentelemetry logs/metrics/traces export
	Enabled bool `yaml:"enabled" env:"ENABLED" example:"false"`
}

type DB struct {
	// Postgres username
	User string `yaml:"user" env:"USER" example:"postgres" validate:"required"`
	// Postgres password
	Pass string `yaml:"pass" env:"PASS" validate:"required"`
	// Postgres host
	Host string `yaml:"host" env:"HOST" example:"localhost:5432" validate:"required"`
	// Postgres database name
	Database string `yaml:"database" env:"DATABASE" example:"hyperfocus" validate:"required"`
}

type Auth struct {
	JWT JWTAuth `yaml:"jwt" envPrefix:"JWT_"`
	// Custom roles object, keys - role names, values - array of granted permissions
	// Example:
	// custom_roles:
	//   admin: ["*"]
	//   vip: ["user:create"]
	//   editor: ["user:*"]
	//   moderator: ["user:*", "all-messages:*"]
	CustomRoles map[string][]api.Permission `yaml:"custom_roles"`
}

type JWTAuth struct {
	// JWT secret, generate it with `openssl rand -base64 32`
	Secret string `yaml:"secret" env:"SECRET" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c" validate:"required"`
}

type Admin struct {
	// Admin password
	Password string `yaml:"password" env:"PASSWORD" example:"qwerty123" validate:"required"`
}

type Twitch struct {
	// ClientID of the twitch application
	ClientID string `yaml:"client_id" example:"a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p" validate:"required"`
	// Client secret of the twitch application
	ClientSecret string `yaml:"client_secret" example:"abc123def456ghi789jkl012mno345pqr678stu901" validate:"required"`
	// Username of the bot account
	Username string `yaml:"username" example:"PogChamp123" validate:"required"`
	// User refresh token of the bot account
	RefreshToken string `yaml:"refresh_token" example:"v1.abc123def456ghi789jkl012mno345pqr678stu901vwx234yz567" validate:"required"`
	// Browser GQL Oauth token
	BrowserOauthToken string `yaml:"browser_oauth_token" example:"v1.abc123def456ghi789jkl012mno345pqr678stu901vwx234yz567" validate:"required"`
}

type Paddle struct {
	// PaddleOCR service base URL
	BaseURL string `yaml:"base_url" example:"http://localhost:5000" validate:"required"`
}

type Processing struct {
	// Number of workers that fetch frames
	FetchWorkerCount int `yaml:"fetch_worker_count" example:"16" validate:"required"`
	// Fetch frame timeout
	FetchTimeout int `yaml:"fetch_timeout" example:"60" validate:"required"`
	// Frame buffer size
	FrameBufferSize int `yaml:"frame_buffer_size" example:"512" validate:"required"`
	// Number of workers that process the frames
	ProcessWorkerCount int `yaml:"process_worker_count" example:"8" validate:"required"`
	// Channel processing timeout in seconds
	ProcessTimeout int `yaml:"process_timeout" example:"60" validate:"required"`
}

type Server struct {
	// Web server port
	HttpPort int `yaml:"http_port" env:"HTTP_PORT" example:"8080" validate:"required"`
}

func Load(configPath string) (*Config, error) {
	var result Config

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, oops.Errorf("failed to read config file: %w", err)
	}
	if err := yaml.Unmarshal(data, &result); err != nil {
		return nil, oops.Errorf("failed to parse YAML config: %w", err)
	}

	if err := env.ParseWithOptions(&result, env.Options{ //nolint:exhaustruct
		Prefix: "HYPERFOCUS_",
	}); err != nil {
		return nil, oops.Errorf("failed to parse environment variables: %w", err)
	}

	if result.ServiceName == "" {
		result.ServiceName = "hyperfocus"
	}
	if result.BaseURL == "" {
		result.BaseURL = "https://hyperfocus.rofleksey.ru"
	}
	if result.DB.User == "" {
		result.DB.User = "postgres"
	}
	if result.DB.Pass == "" {
		result.DB.Pass = "postgres"
	}
	if result.DB.Host == "" {
		result.DB.Host = "localhost:5432"
	}
	if result.DB.Database == "" {
		result.DB.Database = "hyperfocus"
	}
	if result.Processing.ProcessWorkerCount == 0 {
		result.Processing.ProcessWorkerCount = 10
	}
	if result.Processing.ProcessTimeout == 0 {
		result.Processing.ProcessTimeout = 60
	}
	if result.Processing.FetchWorkerCount == 0 {
		result.Processing.FetchWorkerCount = 1
	}
	if result.Processing.FetchTimeout == 0 {
		result.Processing.FetchTimeout = 60
	}
	if result.Processing.FrameBufferSize == 0 {
		result.Processing.FrameBufferSize = 512
	}
	if result.Server.HttpPort == 0 {
		result.Server.HttpPort = 8080
	}
	if result.Paddle.BaseURL == "" {
		result.Paddle.BaseURL = "http://localhost:5000"
	}

	validate := validator.New(validator.WithRequiredStructEnabled())
	if err := validate.Struct(result); err != nil {
		return nil, oops.Errorf("failed to validate config: %w", err)
	}

	return &result, nil
}
