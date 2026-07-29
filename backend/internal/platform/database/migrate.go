package database

import (
	"context"
	"embed"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

const migrationLockKey int64 = 0x4e6f7465466f756e

//go:embed migrations/*.sql
var migrations embed.FS

func ApplyMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	connection, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire migration connection: %w", err)
	}
	defer connection.Release()

	if _, err := connection.Exec(ctx, `SELECT pg_advisory_lock($1)`, migrationLockKey); err != nil {
		return fmt.Errorf("lock migrations: %w", err)
	}
	defer func() {
		_, _ = connection.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, migrationLockKey)
	}()

	if _, err := connection.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version bigint PRIMARY KEY,
			name text NOT NULL,
			applied_at timestamptz NOT NULL DEFAULT now()
		)
	`); err != nil {
		return fmt.Errorf("create schema migration table: %w", err)
	}

	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read embedded migrations: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}
		if err := applyMigration(ctx, connection, entry.Name()); err != nil {
			return err
		}
	}
	return nil
}

func applyMigration(ctx context.Context, connection *pgxpool.Conn, name string) error {
	version, err := migrationVersion(name)
	if err != nil {
		return err
	}

	contents, err := migrations.ReadFile("migrations/" + name)
	if err != nil {
		return fmt.Errorf("read migration %s: %w", name, err)
	}

	transaction, err := connection.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin migration %s: %w", name, err)
	}
	defer func() { _ = transaction.Rollback(context.Background()) }()

	var applied bool
	if err := transaction.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)
	`, version).Scan(&applied); err != nil {
		return fmt.Errorf("check migration %s: %w", name, err)
	}
	if applied {
		return transaction.Commit(ctx)
	}

	if _, err := transaction.Exec(ctx, string(contents)); err != nil {
		return fmt.Errorf("execute migration %s: %w", name, err)
	}
	if _, err := transaction.Exec(ctx, `
		INSERT INTO schema_migrations (version, name) VALUES ($1, $2)
	`, version, name); err != nil {
		return fmt.Errorf("record migration %s: %w", name, err)
	}
	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit migration %s: %w", name, err)
	}
	return nil
}

func migrationVersion(name string) (int64, error) {
	prefix, _, ok := strings.Cut(name, "_")
	if !ok {
		return 0, fmt.Errorf("migration %q must start with a numeric version", name)
	}
	version, err := strconv.ParseInt(prefix, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse migration version %q: %w", name, err)
	}
	return version, nil
}
