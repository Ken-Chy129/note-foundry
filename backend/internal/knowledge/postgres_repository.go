package knowledge

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrSpaceNameConflict = errors.New("knowledge space name already exists")
	ErrSpaceNotFound     = errors.New("knowledge space not found")
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (repository *PostgresRepository) CreateSpace(ctx context.Context, space *Space) error {
	_, err := repository.pool.Exec(ctx, `
		INSERT INTO knowledge_spaces (id, name, visibility)
		VALUES ($1, $2, $3)
	`, space.ID(), space.Name(), space.Visibility())
	if isUniqueViolation(err) {
		return ErrSpaceNameConflict
	}
	if err != nil {
		return fmt.Errorf("insert knowledge space: %w", err)
	}
	return nil
}

func (repository *PostgresRepository) ListSpaces(ctx context.Context, limit, offset int) ([]*Space, int, error) {
	var total int
	if err := repository.pool.QueryRow(ctx, `SELECT count(*) FROM knowledge_spaces`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count knowledge spaces: %w", err)
	}
	rows, err := repository.pool.Query(ctx, `
		SELECT id::text, name, visibility
		FROM knowledge_spaces
		ORDER BY lower(name), id
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("query knowledge spaces: %w", err)
	}
	defer rows.Close()

	spaces := make([]*Space, 0)
	for rows.Next() {
		space, err := scanSpace(rows)
		if err != nil {
			return nil, 0, err
		}
		spaces = append(spaces, space)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate knowledge spaces: %w", err)
	}
	return spaces, total, nil
}

func (repository *PostgresRepository) GetSpace(ctx context.Context, id string) (*Space, error) {
	space, err := scanSpace(repository.pool.QueryRow(ctx, `
		SELECT id::text, name, visibility
		FROM knowledge_spaces
		WHERE id = $1
	`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSpaceNotFound
	}
	if err != nil {
		return nil, err
	}
	return space, nil
}

func (repository *PostgresRepository) UpdateSpace(ctx context.Context, space *Space) error {
	result, err := repository.pool.Exec(ctx, `
		UPDATE knowledge_spaces
		SET name = $2, visibility = $3, updated_at = now()
		WHERE id = $1
	`, space.ID(), space.Name(), space.Visibility())
	if isUniqueViolation(err) {
		return ErrSpaceNameConflict
	}
	if err != nil {
		return fmt.Errorf("update knowledge space: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrSpaceNotFound
	}
	return nil
}

type rowScanner interface {
	Scan(...any) error
}

func scanSpace(row rowScanner) (*Space, error) {
	var id string
	var name string
	var visibility Visibility
	if err := row.Scan(&id, &name, &visibility); err != nil {
		return nil, err
	}
	space, err := NewSpace(id, name, visibility)
	if err != nil {
		return nil, fmt.Errorf("reconstruct knowledge space: %w", err)
	}
	return space, nil
}

func isUniqueViolation(err error) bool {
	var postgresError *pgconn.PgError
	return errors.As(err, &postgresError) && postgresError.Code == "23505"
}
