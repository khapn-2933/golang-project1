package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	App       AppConfig       `mapstructure:"app"`
	Database  DatabaseConfig  `mapstructure:"database"`
	JWT       JWTConfig       `mapstructure:"jwt"`
	OAuth     OAuthConfig     `mapstructure:"oauth"`
	Upload    UploadConfig    `mapstructure:"upload"`
	Email     EmailConfig     `mapstructure:"email"`
	Chatwork  ChatworkConfig  `mapstructure:"chatwork"`
	Scheduler SchedulerConfig `mapstructure:"scheduler"`
}

type EmailConfig struct {
	Enabled           bool   `mapstructure:"enabled"`
	SMTPHost          string `mapstructure:"smtp_host"`
	SMTPPort          int    `mapstructure:"smtp_port"`
	Username          string `mapstructure:"username"`
	Password          string `mapstructure:"password"`
	FromEmail         string `mapstructure:"from_email"`
	FromName          string `mapstructure:"from_name"`
	AdminRecipient    string `mapstructure:"admin_recipient"`
	SubjectPrefix     string `mapstructure:"subject_prefix"`
	OrderTemplatePath string `mapstructure:"order_template_path"`
	MaxRetries        int    `mapstructure:"max_retries"`
	RetryDelaySeconds int    `mapstructure:"retry_delay_seconds"`
	MaxWorkers        int    `mapstructure:"max_workers"`
	QueueSize         int    `mapstructure:"queue_size"`
}

type ChatworkConfig struct {
	Enabled           bool   `mapstructure:"enabled"`
	BaseURL           string `mapstructure:"base_url"`
	APIToken          string `mapstructure:"api_token"`
	RoomID            string `mapstructure:"room_id"`
	MessagePrefix     string `mapstructure:"message_prefix"`
	MaxRetries        int    `mapstructure:"max_retries"`
	RetryDelaySeconds int    `mapstructure:"retry_delay_seconds"`
	MaxWorkers        int    `mapstructure:"max_workers"`
	QueueSize         int    `mapstructure:"queue_size"`
	TimeoutSeconds    int    `mapstructure:"timeout_seconds"`
}

type SchedulerConfig struct {
	Enabled            bool   `mapstructure:"enabled"`
	MonthlyCron        string `mapstructure:"monthly_cron"`
	AdminRecipient     string `mapstructure:"admin_recipient"`
	ReportTemplatePath string `mapstructure:"report_template_path"`
}

type UploadConfig struct {
	Path         string   `mapstructure:"path"`
	MaxSize      int64    `mapstructure:"max_size"`
	AllowedTypes []string `mapstructure:"allowed_types"`
}

type AppConfig struct {
	Name string `mapstructure:"name"`
	Env  string `mapstructure:"env"`
	Port int    `mapstructure:"port"`
}

type DatabaseConfig struct {
	Host            string `mapstructure:"host"`
	Port            int    `mapstructure:"port"`
	Username        string `mapstructure:"username"`
	Password        string `mapstructure:"password"`
	DBName          string `mapstructure:"dbname"`
	Charset         string `mapstructure:"charset"`
	ParseTime       bool   `mapstructure:"parse_time"`
	Loc             string `mapstructure:"loc"`
	MaxIdleConns    int    `mapstructure:"max_idle_conns"`
	MaxOpenConns    int    `mapstructure:"max_open_conns"`
	ConnMaxLifetime int    `mapstructure:"conn_max_lifetime"`
}

type JWTConfig struct {
	Secret     string        `mapstructure:"secret"`
	Expiration time.Duration `mapstructure:"expiration"`
}

// OAuthConfig holds all OAuth provider configurations
type OAuthConfig struct {
	Google   OAuthProviderConfig `mapstructure:"google"`
	Facebook OAuthProviderConfig `mapstructure:"facebook"`
	Twitter  OAuthProviderConfig `mapstructure:"twitter"`
}

// OAuthProviderConfig holds configuration for a single OAuth provider
type OAuthProviderConfig struct {
	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
	RedirectURL  string `mapstructure:"redirect_url"`
}

func LoadConfig(path string) (*Config, error) {
	viper.SetConfigFile(path)
	viper.SetConfigType("yaml")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	applySensitiveEnvOverrides(&config)

	return &config, nil
}

func applySensitiveEnvOverrides(cfg *Config) {
	if cfg == nil {
		return
	}

	if value := strings.TrimSpace(os.Getenv("EMAIL_PASSWORD")); value != "" {
		cfg.Email.Password = value
	}

	if value := strings.TrimSpace(os.Getenv("CHATWORK_API_TOKEN")); value != "" {
		cfg.Chatwork.APIToken = value
	}

	if value := strings.TrimSpace(os.Getenv("DATABASE_PASSWORD")); value != "" {
		cfg.Database.Password = value
	}

	if value := strings.TrimSpace(os.Getenv("JWT_SECRET")); value != "" {
		cfg.JWT.Secret = value
	}
}

func (d *DatabaseConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=%t&loc=%s",
		url.QueryEscape(d.Username),
		url.QueryEscape(d.Password),
		d.Host,
		d.Port,
		d.DBName,
		d.Charset,
		d.ParseTime,
		url.QueryEscape(d.Loc),
	)
}

func (d *DatabaseConfig) MigrationDSN() string {
	return fmt.Sprintf("mysql://%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=%t&loc=%s&multiStatements=true",
		url.QueryEscape(d.Username),
		url.QueryEscape(d.Password),
		d.Host,
		d.Port,
		d.DBName,
		d.Charset,
		d.ParseTime,
		url.QueryEscape(d.Loc),
	)
}
