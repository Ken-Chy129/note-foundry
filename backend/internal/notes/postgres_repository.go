package notes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNoteNotFound     = errors.New("Learning Note not found")
	ErrRevisionNotFound = errors.New("Note Revision not found")
)

type RevisionPage struct {
	Revisions  []Revision
	Page       int
	PageSize   int
	TotalItems int
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (repository *PostgresRepository) CreateNote(ctx context.Context, note *Note) error {
	var directoryID any
	if note.DirectoryID() != "" {
		directoryID = note.DirectoryID()
	}
	if _, err := repository.pool.Exec(ctx, `
		INSERT INTO learning_notes (
			id, space_id, directory_id, title, slug, current_markdown, current_version
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, note.ID(), note.SpaceID(), directoryID, note.Title(), note.Slug(), note.Markdown(), note.Version()); err != nil {
		return fmt.Errorf("insert Learning Note: %w", err)
	}
	return nil
}

func (repository *PostgresRepository) GetNote(ctx context.Context, id string) (*Note, error) {
	note, err := scanNote(repository.pool.QueryRow(ctx, noteSelect+` WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoteNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query Learning Note: %w", err)
	}
	return note, nil
}

func (repository *PostgresRepository) UpdateDraft(ctx context.Context, note *Note, expectedVersion int64) error {
	result, err := repository.pool.Exec(ctx, `
		UPDATE learning_notes
		SET title = $3,
			slug = $4,
			current_markdown = $5,
			current_version = $6,
			updated_at = now()
		WHERE id = $1 AND current_version = $2 AND trashed_at IS NULL
	`, note.ID(), expectedVersion, note.Title(), note.Slug(), note.Markdown(), note.Version())
	if err != nil {
		return fmt.Errorf("update Learning Note draft: %w", err)
	}
	if result.RowsAffected() == 0 {
		return repository.noteMutationMiss(ctx, note.ID())
	}
	return nil
}

func (repository *PostgresRepository) Publish(ctx context.Context, note *Note, revision Revision) error {
	transaction, err := repository.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin publish transaction: %w", err)
	}
	defer func() { _ = transaction.Rollback(context.Background()) }()

	result, err := transaction.Exec(ctx, `
		UPDATE learning_notes
		SET published_title = $3,
			published_slug = $4,
			published_markdown = $5,
			published_at = $6,
			updated_at = now()
		WHERE id = $1 AND current_version = $2 AND trashed_at IS NULL
	`, note.ID(), note.Version(), note.Published().Title, note.Published().Slug, note.Published().Markdown, note.Published().PublishedAt)
	if err != nil {
		return fmt.Errorf("publish Learning Note: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrVersionConflict
	}
	if _, err := transaction.Exec(ctx, `
		INSERT INTO note_revisions (id, note_id, title, slug, markdown, reason, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, revision.ID, revision.NoteID, revision.Title, revision.Slug, revision.Markdown, revision.Reason, revision.CreatedAt); err != nil {
		return fmt.Errorf("insert publish revision: %w", err)
	}
	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit publish transaction: %w", err)
	}
	return nil
}

func (repository *PostgresRepository) CreateCheckpoint(ctx context.Context, noteID string, expectedVersion int64, revision Revision) error {
	result, err := repository.pool.Exec(ctx, `
		INSERT INTO note_revisions (id, note_id, title, slug, markdown, reason, created_at)
		SELECT $3, id, title, slug, current_markdown, $4, $5
		FROM learning_notes
		WHERE id = $1 AND current_version = $2 AND trashed_at IS NULL
	`, noteID, expectedVersion, revision.ID, revision.Reason, revision.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert Note Revision checkpoint: %w", err)
	}
	if result.RowsAffected() == 0 {
		return repository.noteMutationMiss(ctx, noteID)
	}
	return nil
}

func (repository *PostgresRepository) ListRevisions(ctx context.Context, noteID string, page, pageSize int) (RevisionPage, error) {
	var noteExists bool
	if err := repository.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM learning_notes WHERE id = $1)`, noteID).Scan(&noteExists); err != nil {
		return RevisionPage{}, fmt.Errorf("check Learning Note for revisions: %w", err)
	}
	if !noteExists {
		return RevisionPage{}, ErrNoteNotFound
	}
	var totalItems int
	if err := repository.pool.QueryRow(ctx, `SELECT count(*) FROM note_revisions WHERE note_id = $1`, noteID).Scan(&totalItems); err != nil {
		return RevisionPage{}, fmt.Errorf("count Note Revisions: %w", err)
	}
	rows, err := repository.pool.Query(ctx, `
		SELECT id::text, note_id::text, title, slug, markdown, reason, created_at
		FROM note_revisions
		WHERE note_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2 OFFSET $3
	`, noteID, pageSize, (page-1)*pageSize)
	if err != nil {
		return RevisionPage{}, fmt.Errorf("query Note Revisions: %w", err)
	}
	defer rows.Close()
	revisions := make([]Revision, 0, pageSize)
	for rows.Next() {
		revision, err := scanRevision(rows)
		if err != nil {
			return RevisionPage{}, fmt.Errorf("scan Note Revision: %w", err)
		}
		revisions = append(revisions, revision)
	}
	if err := rows.Err(); err != nil {
		return RevisionPage{}, fmt.Errorf("iterate Note Revisions: %w", err)
	}
	return RevisionPage{Revisions: revisions, Page: page, PageSize: pageSize, TotalItems: totalItems}, nil
}

func (repository *PostgresRepository) GetRevision(ctx context.Context, noteID, revisionID string) (Revision, error) {
	revision, err := scanRevision(repository.pool.QueryRow(ctx, `
		SELECT id::text, note_id::text, title, slug, markdown, reason, created_at
		FROM note_revisions
		WHERE note_id = $1 AND id = $2
	`, noteID, revisionID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Revision{}, ErrRevisionNotFound
	}
	if err != nil {
		return Revision{}, fmt.Errorf("query Note Revision: %w", err)
	}
	return revision, nil
}

func (repository *PostgresRepository) Restore(ctx context.Context, note *Note, checkpoint Revision, expectedVersion int64) error {
	transaction, err := repository.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin Note Revision restore: %w", err)
	}
	defer func() { _ = transaction.Rollback(context.Background()) }()
	result, err := transaction.Exec(ctx, `
		UPDATE learning_notes
		SET title = $3,
			slug = $4,
			current_markdown = $5,
			current_version = $6,
			updated_at = now()
		WHERE id = $1 AND current_version = $2 AND trashed_at IS NULL
	`, note.ID(), expectedVersion, note.Title(), note.Slug(), note.Markdown(), note.Version())
	if err != nil {
		return fmt.Errorf("restore Learning Note draft: %w", err)
	}
	if result.RowsAffected() == 0 {
		return repository.noteMutationMiss(ctx, note.ID())
	}
	if _, err := transaction.Exec(ctx, `
		INSERT INTO note_revisions (id, note_id, title, slug, markdown, reason, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, checkpoint.ID, checkpoint.NoteID, checkpoint.Title, checkpoint.Slug, checkpoint.Markdown, checkpoint.Reason, checkpoint.CreatedAt); err != nil {
		return fmt.Errorf("insert pre-restore Note Revision: %w", err)
	}
	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit Note Revision restore: %w", err)
	}
	return nil
}

func (repository *PostgresRepository) noteMutationMiss(ctx context.Context, id string) error {
	var exists bool
	if err := repository.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM learning_notes WHERE id = $1)`, id).Scan(&exists); err != nil {
		return fmt.Errorf("check Learning Note after missed mutation: %w", err)
	}
	if !exists {
		return ErrNoteNotFound
	}
	return ErrVersionConflict
}

const noteSelect = `
	SELECT id::text,
		space_id::text,
		COALESCE(directory_id::text, ''),
		title,
		slug,
		current_markdown,
		current_version,
		published_title,
		published_slug,
		published_markdown,
		published_at
	FROM learning_notes
`

type rowScanner interface {
	Scan(...any) error
}

func scanRevision(row rowScanner) (Revision, error) {
	var revision Revision
	if err := row.Scan(
		&revision.ID,
		&revision.NoteID,
		&revision.Title,
		&revision.Slug,
		&revision.Markdown,
		&revision.Reason,
		&revision.CreatedAt,
	); err != nil {
		return Revision{}, err
	}
	return revision, nil
}

func scanNote(row rowScanner) (*Note, error) {
	var id string
	var spaceID string
	var directoryID string
	var title string
	var slug string
	var markdown string
	var version int64
	var publishedTitle sql.NullString
	var publishedSlug sql.NullString
	var publishedMarkdown sql.NullString
	var publishedAt sql.NullTime
	if err := row.Scan(
		&id,
		&spaceID,
		&directoryID,
		&title,
		&slug,
		&markdown,
		&version,
		&publishedTitle,
		&publishedSlug,
		&publishedMarkdown,
		&publishedAt,
	); err != nil {
		return nil, err
	}
	note, err := NewNote(id, spaceID, directoryID, title, markdown)
	if err != nil {
		return nil, fmt.Errorf("reconstruct Learning Note: %w", err)
	}
	note.slug = slug
	note.version = version
	if publishedAt.Valid {
		note.published = &PublishedContent{
			Title:       publishedTitle.String,
			Slug:        publishedSlug.String,
			Markdown:    publishedMarkdown.String,
			PublishedAt: publishedAt.Time,
		}
	}
	return note, nil
}
