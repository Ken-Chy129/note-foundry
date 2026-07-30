package notes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNoteNotFound          = errors.New("Learning Note not found")
	ErrPublishedNoteNotFound = errors.New("published Learning Note not found")
	ErrRevisionNotFound      = errors.New("Note Revision not found")
	ErrTrashedNoteNotFound   = errors.New("trashed Learning Note not found")
)

type NoteListFilter struct {
	SpaceID     string
	DirectoryID string
	Page        int
	PageSize    int
}

type NotePage struct {
	Notes      []*Note
	Page       int
	PageSize   int
	TotalItems int
}

type NoteSummary struct {
	ID          string
	SpaceID     string
	DirectoryID string
	Title       string
	Slug        string
	Version     int64
	IsPublished bool
	UpdatedAt   time.Time
}

type NoteSummaryPage struct {
	Notes      []NoteSummary
	Page       int
	PageSize   int
	TotalItems int
}

type PublishedNote struct {
	ID          string
	SpaceID     string
	DirectoryID string
	Title       string
	Slug        string
	Markdown    string
	PublishedAt sql.NullTime
}

type PublishedNotePage struct {
	Notes      []PublishedNote
	Page       int
	PageSize   int
	TotalItems int
}

type TrashEntry struct {
	Note      *Note
	TrashedAt time.Time
}

type TrashPage struct {
	Entries    []TrashEntry
	Page       int
	PageSize   int
	TotalItems int
}

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
	note, err := scanNote(repository.pool.QueryRow(ctx, noteSelect+` WHERE id = $1 AND trashed_at IS NULL`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoteNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query Learning Note: %w", err)
	}
	return note, nil
}

func (repository *PostgresRepository) TrashNote(ctx context.Context, id string, expectedVersion int64, trashedAt time.Time) error {
	result, err := repository.pool.Exec(ctx, `
		UPDATE learning_notes
		SET trashed_at = $3, updated_at = now()
		WHERE id = $1 AND current_version = $2 AND trashed_at IS NULL
	`, id, expectedVersion, trashedAt)
	if err != nil {
		return fmt.Errorf("trash Learning Note: %w", err)
	}
	if result.RowsAffected() == 0 {
		return repository.noteMutationMiss(ctx, id)
	}
	return nil
}

func (repository *PostgresRepository) ListTrash(ctx context.Context, page, pageSize int) (TrashPage, error) {
	var totalItems int
	if err := repository.pool.QueryRow(ctx, `SELECT count(*) FROM learning_notes WHERE trashed_at IS NOT NULL`).Scan(&totalItems); err != nil {
		return TrashPage{}, fmt.Errorf("count trashed Learning Notes: %w", err)
	}
	rows, err := repository.pool.Query(ctx, trashedNoteSelect+`
		WHERE trashed_at IS NOT NULL
		ORDER BY trashed_at DESC, id DESC
		LIMIT $1 OFFSET $2
	`, pageSize, (page-1)*pageSize)
	if err != nil {
		return TrashPage{}, fmt.Errorf("query trashed Learning Notes: %w", err)
	}
	defer rows.Close()
	entries := make([]TrashEntry, 0, pageSize)
	for rows.Next() {
		entry, err := scanTrashEntry(rows)
		if err != nil {
			return TrashPage{}, fmt.Errorf("scan trashed Learning Note: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return TrashPage{}, fmt.Errorf("iterate trashed Learning Notes: %w", err)
	}
	return TrashPage{Entries: entries, Page: page, PageSize: pageSize, TotalItems: totalItems}, nil
}

func (repository *PostgresRepository) GetTrashedNote(ctx context.Context, id string) (TrashEntry, error) {
	entry, err := scanTrashEntry(repository.pool.QueryRow(ctx, trashedNoteSelect+` WHERE id = $1 AND trashed_at IS NOT NULL`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return TrashEntry{}, ErrTrashedNoteNotFound
	}
	if err != nil {
		return TrashEntry{}, fmt.Errorf("query trashed Learning Note: %w", err)
	}
	return entry, nil
}

func (repository *PostgresRepository) RestoreFromTrash(ctx context.Context, note *Note, revision *Revision) error {
	transaction, err := repository.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin trash restore: %w", err)
	}
	defer func() { _ = transaction.Rollback(context.Background()) }()
	var directoryID any
	if note.DirectoryID() != "" {
		directoryID = note.DirectoryID()
	}
	var publishedTitle any
	var publishedSlug any
	var publishedMarkdown any
	var publishedAt any
	if note.Published() != nil {
		publishedTitle = note.Published().Title
		publishedSlug = note.Published().Slug
		publishedMarkdown = note.Published().Markdown
		publishedAt = note.Published().PublishedAt
	}
	result, err := transaction.Exec(ctx, `
		UPDATE learning_notes
		SET space_id = $2,
			directory_id = $3,
			published_title = $4,
			published_slug = $5,
			published_markdown = $6,
			published_at = $7,
			trashed_at = NULL,
			updated_at = now()
		WHERE id = $1 AND trashed_at IS NOT NULL
	`, note.ID(), note.SpaceID(), directoryID, publishedTitle, publishedSlug, publishedMarkdown, publishedAt)
	if err != nil {
		return fmt.Errorf("restore Learning Note from Trash: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrTrashedNoteNotFound
	}
	if revision != nil {
		if _, err := transaction.Exec(ctx, `
			INSERT INTO note_revisions (id, note_id, title, slug, markdown, reason, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, revision.ID, revision.NoteID, revision.Title, revision.Slug, revision.Markdown, revision.Reason, revision.CreatedAt); err != nil {
			return fmt.Errorf("insert public trash restore revision: %w", err)
		}
	}
	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit trash restore: %w", err)
	}
	return nil
}

func (repository *PostgresRepository) DeleteTrashedNote(ctx context.Context, id string) error {
	result, err := repository.pool.Exec(ctx, `DELETE FROM learning_notes WHERE id = $1 AND trashed_at IS NOT NULL`, id)
	if err != nil {
		return fmt.Errorf("permanently delete Learning Note: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrTrashedNoteNotFound
	}
	return nil
}

func (repository *PostgresRepository) ListNotes(ctx context.Context, filter NoteListFilter) (NotePage, error) {
	spaceID := nullableString(filter.SpaceID)
	directoryID := nullableString(filter.DirectoryID)
	var totalItems int
	if err := repository.pool.QueryRow(ctx, `
		SELECT count(*)
		FROM learning_notes
		WHERE trashed_at IS NULL
			AND ($1::uuid IS NULL OR space_id = $1)
			AND ($2::uuid IS NULL OR directory_id = $2)
	`, spaceID, directoryID).Scan(&totalItems); err != nil {
		return NotePage{}, fmt.Errorf("count Learning Notes: %w", err)
	}
	rows, err := repository.pool.Query(ctx, noteSelect+`
		WHERE trashed_at IS NULL
			AND ($1::uuid IS NULL OR space_id = $1)
			AND ($2::uuid IS NULL OR directory_id = $2)
		ORDER BY updated_at DESC, id DESC
		LIMIT $3 OFFSET $4
	`, spaceID, directoryID, filter.PageSize, (filter.Page-1)*filter.PageSize)
	if err != nil {
		return NotePage{}, fmt.Errorf("query Learning Notes: %w", err)
	}
	defer rows.Close()
	notes := make([]*Note, 0, filter.PageSize)
	for rows.Next() {
		note, err := scanNote(rows)
		if err != nil {
			return NotePage{}, fmt.Errorf("scan Learning Note: %w", err)
		}
		notes = append(notes, note)
	}
	if err := rows.Err(); err != nil {
		return NotePage{}, fmt.Errorf("iterate Learning Notes: %w", err)
	}
	return NotePage{Notes: notes, Page: filter.Page, PageSize: filter.PageSize, TotalItems: totalItems}, nil
}

func (repository *PostgresRepository) ListNoteSummaries(ctx context.Context, filter NoteListFilter) (NoteSummaryPage, error) {
	spaceID := nullableString(filter.SpaceID)
	directoryID := nullableString(filter.DirectoryID)
	var totalItems int
	if err := repository.pool.QueryRow(ctx, `
		SELECT count(*)
		FROM learning_notes
		WHERE trashed_at IS NULL
			AND ($1::uuid IS NULL OR space_id = $1)
			AND ($2::uuid IS NULL OR directory_id = $2)
	`, spaceID, directoryID).Scan(&totalItems); err != nil {
		return NoteSummaryPage{}, fmt.Errorf("count Learning Note summaries: %w", err)
	}
	rows, err := repository.pool.Query(ctx, `
		SELECT id::text,
			space_id::text,
			COALESCE(directory_id::text, ''),
			title,
			slug,
			current_version,
			published_at IS NOT NULL,
			updated_at
		FROM learning_notes
		WHERE trashed_at IS NULL
			AND ($1::uuid IS NULL OR space_id = $1)
			AND ($2::uuid IS NULL OR directory_id = $2)
		ORDER BY updated_at DESC, id DESC
		LIMIT $3 OFFSET $4
	`, spaceID, directoryID, filter.PageSize, (filter.Page-1)*filter.PageSize)
	if err != nil {
		return NoteSummaryPage{}, fmt.Errorf("query Learning Note summaries: %w", err)
	}
	defer rows.Close()
	summaries := make([]NoteSummary, 0, filter.PageSize)
	for rows.Next() {
		var summary NoteSummary
		if err := rows.Scan(
			&summary.ID,
			&summary.SpaceID,
			&summary.DirectoryID,
			&summary.Title,
			&summary.Slug,
			&summary.Version,
			&summary.IsPublished,
			&summary.UpdatedAt,
		); err != nil {
			return NoteSummaryPage{}, fmt.Errorf("scan Learning Note summary: %w", err)
		}
		summaries = append(summaries, summary)
	}
	if err := rows.Err(); err != nil {
		return NoteSummaryPage{}, fmt.Errorf("iterate Learning Note summaries: %w", err)
	}
	return NoteSummaryPage{Notes: summaries, Page: filter.Page, PageSize: filter.PageSize, TotalItems: totalItems}, nil
}

func (repository *PostgresRepository) GetPublishedNote(ctx context.Context, id string) (PublishedNote, error) {
	note, err := scanPublishedNote(repository.pool.QueryRow(ctx, publishedNoteSelect+`
		WHERE learning_notes.id = $1
			AND learning_notes.trashed_at IS NULL
			AND knowledge_spaces.visibility = 'public'
			AND learning_notes.published_at IS NOT NULL
	`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return PublishedNote{}, ErrPublishedNoteNotFound
	}
	if err != nil {
		return PublishedNote{}, fmt.Errorf("query published Learning Note: %w", err)
	}
	return note, nil
}

func (repository *PostgresRepository) ListPublishedNotes(ctx context.Context, spaceID string, page, pageSize int) (PublishedNotePage, error) {
	var totalItems int
	if err := repository.pool.QueryRow(ctx, `
		SELECT count(*)
		FROM learning_notes
		JOIN knowledge_spaces ON knowledge_spaces.id = learning_notes.space_id
		WHERE learning_notes.space_id = $1
			AND learning_notes.trashed_at IS NULL
			AND knowledge_spaces.visibility = 'public'
			AND learning_notes.published_at IS NOT NULL
	`, spaceID).Scan(&totalItems); err != nil {
		return PublishedNotePage{}, fmt.Errorf("count published Learning Notes: %w", err)
	}
	rows, err := repository.pool.Query(ctx, publishedNoteSelect+`
		WHERE learning_notes.space_id = $1
			AND learning_notes.trashed_at IS NULL
			AND knowledge_spaces.visibility = 'public'
			AND learning_notes.published_at IS NOT NULL
		ORDER BY learning_notes.published_at DESC, learning_notes.id DESC
		LIMIT $2 OFFSET $3
	`, spaceID, pageSize, (page-1)*pageSize)
	if err != nil {
		return PublishedNotePage{}, fmt.Errorf("query published Learning Notes: %w", err)
	}
	defer rows.Close()
	notes := make([]PublishedNote, 0, pageSize)
	for rows.Next() {
		note, err := scanPublishedNote(rows)
		if err != nil {
			return PublishedNotePage{}, fmt.Errorf("scan published Learning Note: %w", err)
		}
		notes = append(notes, note)
	}
	if err := rows.Err(); err != nil {
		return PublishedNotePage{}, fmt.Errorf("iterate published Learning Notes: %w", err)
	}
	return PublishedNotePage{Notes: notes, Page: page, PageSize: pageSize, TotalItems: totalItems}, nil
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

func (repository *PostgresRepository) Move(ctx context.Context, note *Note, revision *Revision, expectedVersion int64) error {
	transaction, err := repository.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin Learning Note move: %w", err)
	}
	defer func() { _ = transaction.Rollback(context.Background()) }()
	var directoryID any
	if note.DirectoryID() != "" {
		directoryID = note.DirectoryID()
	}
	var publishedTitle any
	var publishedSlug any
	var publishedMarkdown any
	var publishedAt any
	if note.Published() != nil {
		publishedTitle = note.Published().Title
		publishedSlug = note.Published().Slug
		publishedMarkdown = note.Published().Markdown
		publishedAt = note.Published().PublishedAt
	}
	result, err := transaction.Exec(ctx, `
		UPDATE learning_notes
		SET space_id = $3,
			directory_id = $4,
			published_title = $5,
			published_slug = $6,
			published_markdown = $7,
			published_at = $8,
			updated_at = now()
		WHERE id = $1 AND current_version = $2 AND trashed_at IS NULL
	`, note.ID(), expectedVersion, note.SpaceID(), directoryID, publishedTitle, publishedSlug, publishedMarkdown, publishedAt)
	if err != nil {
		return fmt.Errorf("update Learning Note location: %w", err)
	}
	if result.RowsAffected() == 0 {
		return repository.noteMutationMiss(ctx, note.ID())
	}
	if revision != nil {
		if _, err := transaction.Exec(ctx, `
			INSERT INTO note_revisions (id, note_id, title, slug, markdown, reason, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, revision.ID, revision.NoteID, revision.Title, revision.Slug, revision.Markdown, revision.Reason, revision.CreatedAt); err != nil {
			return fmt.Errorf("insert private-to-public move revision: %w", err)
		}
	}
	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit Learning Note move: %w", err)
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
		published_at,
		updated_at
	FROM learning_notes
`

const publishedNoteSelect = `
	SELECT learning_notes.id::text,
		learning_notes.space_id::text,
		COALESCE(learning_notes.directory_id::text, ''),
		learning_notes.published_title,
		learning_notes.published_slug,
		learning_notes.published_markdown,
		learning_notes.published_at
	FROM learning_notes
	JOIN knowledge_spaces ON knowledge_spaces.id = learning_notes.space_id
`

const trashedNoteSelect = `
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
		published_at,
		updated_at,
		trashed_at
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

func scanPublishedNote(row rowScanner) (PublishedNote, error) {
	var note PublishedNote
	if err := row.Scan(
		&note.ID,
		&note.SpaceID,
		&note.DirectoryID,
		&note.Title,
		&note.Slug,
		&note.Markdown,
		&note.PublishedAt,
	); err != nil {
		return PublishedNote{}, err
	}
	return note, nil
}

func scanTrashEntry(row rowScanner) (TrashEntry, error) {
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
	var updatedAt time.Time
	var trashedAt time.Time
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
		&updatedAt,
		&trashedAt,
	); err != nil {
		return TrashEntry{}, err
	}
	note, err := NewNote(id, spaceID, directoryID, title, markdown)
	if err != nil {
		return TrashEntry{}, fmt.Errorf("reconstruct trashed Learning Note: %w", err)
	}
	note.slug = slug
	note.version = version
	note.updatedAt = updatedAt
	if publishedAt.Valid {
		note.published = &PublishedContent{
			Title:       publishedTitle.String,
			Slug:        publishedSlug.String,
			Markdown:    publishedMarkdown.String,
			PublishedAt: publishedAt.Time,
		}
	}
	return TrashEntry{Note: note, TrashedAt: trashedAt}, nil
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
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
	var updatedAt time.Time
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
		&updatedAt,
	); err != nil {
		return nil, err
	}
	note, err := NewNote(id, spaceID, directoryID, title, markdown)
	if err != nil {
		return nil, fmt.Errorf("reconstruct Learning Note: %w", err)
	}
	note.slug = slug
	note.version = version
	note.updatedAt = updatedAt
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
