package database

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestApplyMigrationsIsRepeatable(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer pool.Close()

	if err := ApplyMigrations(ctx, pool); err != nil {
		t.Fatalf("first ApplyMigrations() error = %v", err)
	}
	if err := ApplyMigrations(ctx, pool); err != nil {
		t.Fatalf("second ApplyMigrations() error = %v", err)
	}

	var extensionInstalled bool
	if err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM pg_extension WHERE extname = 'pg_trgm'
		)
	`).Scan(&extensionInstalled); err != nil {
		t.Fatalf("query pg_trgm extension: %v", err)
	}
	if !extensionInstalled {
		t.Error("pg_trgm extension is not installed")
	}

	var appliedCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&appliedCount); err != nil {
		t.Fatalf("count schema migrations: %v", err)
	}
	if appliedCount != 1 {
		t.Errorf("schema migration count = %d, want 1", appliedCount)
	}
}
