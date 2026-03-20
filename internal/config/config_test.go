package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestDatabaseConfigDSN(t *testing.T) {
	// Uses process environment via t.Setenv; do not run in parallel.
	t.Setenv("TEST_DATABASE_HOST", "127.0.0.1")
	t.Setenv("TEST_DATABASE_PORT", "3306")
	t.Setenv("TEST_DATABASE_USERNAME", "user name")
	t.Setenv("TEST_DATABASE_PASSWORD", "p@ss word")

	port, err := strconv.Atoi(os.Getenv("TEST_DATABASE_PORT"))
	if err != nil {
		t.Fatalf("parse TEST_DATABASE_PORT: %v", err)
	}

	db := &DatabaseConfig{
		Host:      os.Getenv("TEST_DATABASE_HOST"),
		Port:      port,
		Username:  os.Getenv("TEST_DATABASE_USERNAME"),
		Password:  os.Getenv("TEST_DATABASE_PASSWORD"),
		DBName:    "foods",
		Charset:   "utf8mb4",
		ParseTime: true,
		Loc:       "Asia/Ho_Chi_Minh",
	}

	dsn := db.DSN()
	if !strings.Contains(dsn, "user+name:p%40ss+word@tcp(127.0.0.1:3306)/foods") {
		t.Fatalf("unexpected DSN: %s", dsn)
	}
	if !strings.Contains(dsn, "loc=Asia%2FHo_Chi_Minh") {
		t.Fatalf("expected escaped location in DSN: %s", dsn)
	}

	mig := db.MigrationDSN()
	if !strings.HasPrefix(mig, "mysql://") {
		t.Fatalf("unexpected migration DSN prefix: %s", mig)
	}
	if !strings.Contains(mig, "multiStatements=true") {
		t.Fatalf("migration DSN missing multiStatements: %s", mig)
	}
}

func TestApplySensitiveEnvOverrides(t *testing.T) {
	cfg := &Config{}
	cfg.Email.Password = "old-email"
	cfg.Chatwork.APIToken = "old-chat"
	cfg.Database.Password = "old-db"
	cfg.JWT.Secret = "old-jwt"

	t.Setenv("EMAIL_PASSWORD", "new-email")
	t.Setenv("CHATWORK_API_TOKEN", "new-chat")
	t.Setenv("DATABASE_PASSWORD", "new-db")
	t.Setenv("JWT_SECRET", "new-jwt")

	applySensitiveEnvOverrides(cfg)

	if cfg.Email.Password != "new-email" {
		t.Fatalf("email password = %s", cfg.Email.Password)
	}
	if cfg.Chatwork.APIToken != "new-chat" {
		t.Fatalf("chatwork api token = %s", cfg.Chatwork.APIToken)
	}
	if cfg.Database.Password != "new-db" {
		t.Fatalf("db password = %s", cfg.Database.Password)
	}
	if cfg.JWT.Secret != "new-jwt" {
		t.Fatalf("jwt secret = %s", cfg.JWT.Secret)
	}
}

func TestLoadConfig_WithEnvOverrides(t *testing.T) {
	// Uses process environment via t.Setenv; do not run in parallel.
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	content := `app:
  name: foods
  env: test
  port: 8080
  base_url: http://localhost:8080
database:
  host: 127.0.0.1
  port: 3306
  username: root
  password: from-file
  dbname: foods
  charset: utf8mb4
  parse_time: true
  loc: Local
  max_idle_conns: 5
  max_open_conns: 10
  conn_max_lifetime: 30
jwt:
  secret: file-secret
  expiration: 1h
email:
  password: file-email-password
chatwork:
  api_token: file-chat-token
`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatalf("write temp config: %v", err)
	}

	t.Setenv("JWT_SECRET", "env-secret")
	t.Setenv("DATABASE_PASSWORD", "env-db-password")
	t.Setenv("EMAIL_PASSWORD", "env-email-password")
	t.Setenv("CHATWORK_API_TOKEN", "env-chat-token")

	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.App.Name != "foods" {
		t.Fatalf("app name = %s", cfg.App.Name)
	}
	if cfg.JWT.Secret != "env-secret" {
		t.Fatalf("jwt secret should be overridden by env, got %s", cfg.JWT.Secret)
	}
	if cfg.Database.Password != "env-db-password" {
		t.Fatalf("db password should be overridden by env, got %s", cfg.Database.Password)
	}
	if cfg.Email.Password != "env-email-password" {
		t.Fatalf("email password should be overridden by env, got %s", cfg.Email.Password)
	}
	if cfg.Chatwork.APIToken != "env-chat-token" {
		t.Fatalf("chatwork token should be overridden by env, got %s", cfg.Chatwork.APIToken)
	}
}
