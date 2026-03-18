package database

import (
	"testing"

	"github.com/kha/foods-drinks/internal/config"
)

func TestNewConnection_InvalidConfig(t *testing.T) {
	t.Parallel()

	cfg := &config.DatabaseConfig{
		Host:            "127.0.0.1",
		Port:            65500,
		Username:        "invalid",
		Password:        "invalid",
		DBName:          "invalid",
		Charset:         "utf8mb4",
		ParseTime:       true,
		Loc:             "Local",
		MaxIdleConns:    1,
		MaxOpenConns:    1,
		ConnMaxLifetime: 1,
	}

	db, err := NewConnection(cfg)
	if err == nil {
		t.Fatalf("expected connection error, got db=%v", db)
	}
}
