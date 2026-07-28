package attachments

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrAttachmentNotFound     = errors.New("Attachment not found")
	ErrAttachmentNoteNotFound = errors.New("Learning Note not found for Attachment")
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (repository *PostgresRepository) NoteExists(ctx context.Context, noteID string) (bool, error) {
	var exists bool
	if err := repository.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM learning_notes WHERE id = $1 AND trashed_at IS NULL)`, noteID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check Attachment Learning Note: %w", err)
	}
	return exists, nil
}

func (repository *PostgresRepository) Create(ctx context.Context, attachment Attachment) error {
	_, err := repository.pool.Exec(ctx, `
		INSERT INTO attachments (
			id, note_id, storage_key, original_name, media_type, size_bytes, sha256_hex, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, attachment.ID, attachment.NoteID, attachment.StorageKey, attachment.OriginalName, attachment.MediaType, attachment.SizeBytes, attachment.SHA256Hex, attachment.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert Attachment metadata: %w", err)
	}
	return nil
}

func (repository *PostgresRepository) ListForNote(ctx context.Context, noteID string) ([]Attachment, error) {
	exists, err := repository.NoteExists(ctx, noteID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrAttachmentNoteNotFound
	}
	rows, err := repository.pool.Query(ctx, attachmentSelect+`
		WHERE attachments.note_id = $1
		ORDER BY attachments.created_at, attachments.id
	`, noteID)
	if err != nil {
		return nil, fmt.Errorf("query Learning Note Attachments: %w", err)
	}
	defer rows.Close()
	attachments := make([]Attachment, 0)
	for rows.Next() {
		attachment, err := scanAttachment(rows)
		if err != nil {
			return nil, fmt.Errorf("scan Attachment: %w", err)
		}
		attachments = append(attachments, attachment)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate Attachments: %w", err)
	}
	return attachments, nil
}

func (repository *PostgresRepository) GetOwner(ctx context.Context, id string) (Attachment, error) {
	attachment, err := scanAttachment(repository.pool.QueryRow(ctx, attachmentSelect+` WHERE attachments.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Attachment{}, ErrAttachmentNotFound
	}
	if err != nil {
		return Attachment{}, fmt.Errorf("query owner Attachment: %w", err)
	}
	return attachment, nil
}

func (repository *PostgresRepository) GetPublic(ctx context.Context, id string) (Attachment, error) {
	attachment, err := scanAttachment(repository.pool.QueryRow(ctx, attachmentSelect+`
		JOIN learning_notes ON learning_notes.id = attachments.note_id
		JOIN knowledge_spaces ON knowledge_spaces.id = learning_notes.space_id
		WHERE attachments.id = $1
			AND attachments.published_at IS NOT NULL
			AND learning_notes.trashed_at IS NULL
			AND learning_notes.published_at IS NOT NULL
			AND knowledge_spaces.visibility = 'public'
	`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Attachment{}, ErrAttachmentNotFound
	}
	if err != nil {
		return Attachment{}, fmt.Errorf("query public Attachment: %w", err)
	}
	return attachment, nil
}

func (repository *PostgresRepository) ReplacePublishedNoteAttachments(ctx context.Context, noteID string, attachmentIDs []string, publishedAt time.Time) error {
	transaction, err := repository.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin Attachment publication: %w", err)
	}
	defer func() { _ = transaction.Rollback(context.Background()) }()
	if _, err := transaction.Exec(ctx, `UPDATE attachments SET published_at = NULL WHERE note_id = $1`, noteID); err != nil {
		return fmt.Errorf("clear published Attachments: %w", err)
	}
	if len(attachmentIDs) > 0 {
		if _, err := transaction.Exec(ctx, `
			UPDATE attachments
			SET published_at = $3
			WHERE note_id = $1 AND id = ANY($2::uuid[])
		`, noteID, attachmentIDs, publishedAt); err != nil {
			return fmt.Errorf("publish referenced Attachments: %w", err)
		}
	}
	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit Attachment publication: %w", err)
	}
	return nil
}

const attachmentSelect = `
	SELECT attachments.id::text,
		attachments.note_id::text,
		attachments.storage_key,
		attachments.original_name,
		attachments.media_type,
		attachments.size_bytes,
		attachments.sha256_hex,
		attachments.published_at,
		attachments.created_at
	FROM attachments
`

type rowScanner interface {
	Scan(...any) error
}

func scanAttachment(row rowScanner) (Attachment, error) {
	var attachment Attachment
	var publishedAt *time.Time
	if err := row.Scan(
		&attachment.ID,
		&attachment.NoteID,
		&attachment.StorageKey,
		&attachment.OriginalName,
		&attachment.MediaType,
		&attachment.SizeBytes,
		&attachment.SHA256Hex,
		&publishedAt,
		&attachment.CreatedAt,
	); err != nil {
		return Attachment{}, err
	}
	attachment.PublishedAt = publishedAt
	return attachment, nil
}
