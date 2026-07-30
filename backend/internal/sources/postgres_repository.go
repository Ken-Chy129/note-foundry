package sources

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (repository *PostgresRepository) CreateSource(ctx context.Context, source *Source) error {
	var spaceID any
	if source.SpaceID() != "" {
		spaceID = source.SpaceID()
	}
	_, err := repository.pool.Exec(ctx, `
		INSERT INTO learning_sources (
			id, kind, space_id, title, capture_note, current_content,
			processing_status, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, source.ID(), source.Kind(), spaceID, source.Title(), source.CaptureNote(), source.Content(), source.ProcessingStatus(), source.CreatedAt(), source.UpdatedAt())
	if err != nil {
		return fmt.Errorf("insert Learning Source: %w", err)
	}
	return nil
}

func (repository *PostgresRepository) GetSource(ctx context.Context, id string) (*Source, error) {
	source, err := scanSource(repository.pool.QueryRow(ctx, sourceSelect+` WHERE id = $1 AND trashed_at IS NULL`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSourceNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query Learning Source: %w", err)
	}
	return source, nil
}

func (repository *PostgresRepository) UpdateSourceOrganization(ctx context.Context, source *Source) error {
	var spaceID any
	if source.SpaceID() != "" {
		spaceID = source.SpaceID()
	}
	command, err := repository.pool.Exec(ctx, `
		UPDATE learning_sources
		SET space_id = $2,
			updated_at = $3
		WHERE id = $1
			AND trashed_at IS NULL
	`, source.ID(), spaceID, source.UpdatedAt())
	if err != nil {
		return fmt.Errorf("update Learning Source: %w", err)
	}
	if command.RowsAffected() == 0 {
		return ErrSourceNotFound
	}
	return nil
}

func (repository *PostgresRepository) ListSources(ctx context.Context, filter ListFilter) (SourcePage, error) {
	var spaceID any
	if filter.SpaceID != "" {
		spaceID = filter.SpaceID
	}
	var totalItems int
	if err := repository.pool.QueryRow(ctx, `
		SELECT count(*)
		FROM learning_sources
		WHERE trashed_at IS NULL
			AND (NOT $1::boolean OR space_id IS NULL)
			AND ($2::uuid IS NULL OR space_id = $2)
	`, filter.InboxOnly, spaceID).Scan(&totalItems); err != nil {
		return SourcePage{}, fmt.Errorf("count Learning Sources: %w", err)
	}
	rows, err := repository.pool.Query(ctx, `
		SELECT id::text,
			kind,
			COALESCE(space_id::text, ''),
			title,
			capture_note,
			processing_status,
			created_at,
			updated_at
		FROM learning_sources
		WHERE trashed_at IS NULL
			AND (NOT $1::boolean OR space_id IS NULL)
			AND ($2::uuid IS NULL OR space_id = $2)
		ORDER BY updated_at DESC, id DESC
		LIMIT $3 OFFSET $4
	`, filter.InboxOnly, spaceID, filter.PageSize, (filter.Page-1)*filter.PageSize)
	if err != nil {
		return SourcePage{}, fmt.Errorf("query Learning Sources: %w", err)
	}
	defer rows.Close()
	sources := make([]SourceSummary, 0, filter.PageSize)
	for rows.Next() {
		var source SourceSummary
		if err := rows.Scan(
			&source.ID,
			&source.Kind,
			&source.SpaceID,
			&source.Title,
			&source.CaptureNote,
			&source.ProcessingStatus,
			&source.CreatedAt,
			&source.UpdatedAt,
		); err != nil {
			return SourcePage{}, fmt.Errorf("scan Learning Source summary: %w", err)
		}
		sources = append(sources, source)
	}
	if err := rows.Err(); err != nil {
		return SourcePage{}, fmt.Errorf("iterate Learning Sources: %w", err)
	}
	return SourcePage{Sources: sources, Page: filter.Page, PageSize: filter.PageSize, TotalItems: totalItems}, nil
}

const sourceSelect = `
	SELECT id::text,
		kind,
		COALESCE(space_id::text, ''),
		title,
		capture_note,
		current_content,
		processing_status,
		created_at,
		updated_at
	FROM learning_sources
`

type rowScanner interface {
	Scan(...any) error
}

func scanSource(row rowScanner) (*Source, error) {
	var source Source
	if err := row.Scan(
		&source.id,
		&source.kind,
		&source.spaceID,
		&source.title,
		&source.captureNote,
		&source.content,
		&source.processingStatus,
		&source.createdAt,
		&source.updatedAt,
	); err != nil {
		return nil, err
	}
	return &source, nil
}
