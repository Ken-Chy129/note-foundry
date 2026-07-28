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
	ErrSpaceNameConflict      = errors.New("knowledge space name already exists")
	ErrSpaceNotFound          = errors.New("knowledge space not found")
	ErrDirectoryNameConflict  = errors.New("directory name already exists under this parent")
	ErrDirectoryNotFound      = errors.New("directory not found")
	ErrDirectoryParentInvalid = errors.New("directory parent does not exist in the same Knowledge Space")
	ErrTagNameConflict        = errors.New("tag name already exists")
	ErrTagNotFound            = errors.New("tag not found")
	ErrTaggableNoteNotFound   = errors.New("Learning Note not found for tag association")
	ErrTagMergeSame           = errors.New("a Tag cannot be merged into itself")
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

func (repository *PostgresRepository) ListPublicSpaces(ctx context.Context, limit, offset int) ([]*Space, int, error) {
	var total int
	if err := repository.pool.QueryRow(ctx, `SELECT count(*) FROM knowledge_spaces WHERE visibility = 'public'`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count public Knowledge Spaces: %w", err)
	}
	rows, err := repository.pool.Query(ctx, `
		SELECT id::text, name, visibility
		FROM knowledge_spaces
		WHERE visibility = 'public'
		ORDER BY lower(name), id
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("query public Knowledge Spaces: %w", err)
	}
	defer rows.Close()
	spaces := make([]*Space, 0, limit)
	for rows.Next() {
		space, err := scanSpace(rows)
		if err != nil {
			return nil, 0, err
		}
		spaces = append(spaces, space)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate public Knowledge Spaces: %w", err)
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

func (repository *PostgresRepository) GetPublicSpace(ctx context.Context, id string) (*Space, error) {
	space, err := scanSpace(repository.pool.QueryRow(ctx, `
		SELECT id::text, name, visibility
		FROM knowledge_spaces
		WHERE id = $1 AND visibility = 'public'
	`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSpaceNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query public Knowledge Space: %w", err)
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

func (repository *PostgresRepository) CreateDirectory(ctx context.Context, directory *Directory) error {
	var parentID any
	if directory.ParentID() != "" {
		parentID = directory.ParentID()
	}
	_, err := repository.pool.Exec(ctx, `
		INSERT INTO directories (id, space_id, parent_id, name)
		VALUES ($1, $2, $3, $4)
	`, directory.ID(), directory.SpaceID(), parentID, directory.Name())
	if isUniqueViolation(err) {
		return ErrDirectoryNameConflict
	}
	if isForeignKeyViolation(err) {
		return ErrDirectoryParentInvalid
	}
	if err != nil {
		return fmt.Errorf("insert directory: %w", err)
	}
	return nil
}

func (repository *PostgresRepository) ListDirectories(ctx context.Context, spaceID string) ([]*Directory, error) {
	rows, err := repository.pool.Query(ctx, `
		SELECT id::text, space_id::text, COALESCE(parent_id::text, ''), name
		FROM directories
		WHERE space_id = $1
		ORDER BY parent_id NULLS FIRST, lower(name), id
	`, spaceID)
	if err != nil {
		return nil, fmt.Errorf("query directories: %w", err)
	}
	defer rows.Close()

	directories := make([]*Directory, 0)
	for rows.Next() {
		directory, err := scanDirectory(rows)
		if err != nil {
			return nil, err
		}
		directories = append(directories, directory)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate directories: %w", err)
	}
	return directories, nil
}

func (repository *PostgresRepository) ListPublicDirectories(ctx context.Context, spaceID string) ([]*Directory, error) {
	rows, err := repository.pool.Query(ctx, `
		SELECT directories.id::text, directories.space_id::text, COALESCE(directories.parent_id::text, ''), directories.name
		FROM directories
		JOIN knowledge_spaces ON knowledge_spaces.id = directories.space_id
		WHERE directories.space_id = $1 AND knowledge_spaces.visibility = 'public'
		ORDER BY directories.parent_id NULLS FIRST, lower(directories.name), directories.id
	`, spaceID)
	if err != nil {
		return nil, fmt.Errorf("query public directories: %w", err)
	}
	defer rows.Close()
	directories := make([]*Directory, 0)
	for rows.Next() {
		directory, err := scanDirectory(rows)
		if err != nil {
			return nil, err
		}
		directories = append(directories, directory)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate public directories: %w", err)
	}
	return directories, nil
}

func (repository *PostgresRepository) GetDirectory(ctx context.Context, id string) (*Directory, error) {
	directory, err := scanDirectory(repository.pool.QueryRow(ctx, `
		SELECT id::text, space_id::text, COALESCE(parent_id::text, ''), name
		FROM directories
		WHERE id = $1
	`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrDirectoryNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query directory: %w", err)
	}
	return directory, nil
}

func (repository *PostgresRepository) UpdateDirectory(ctx context.Context, directory *Directory) error {
	var parentID any
	if directory.ParentID() != "" {
		parentID = directory.ParentID()
	}
	result, err := repository.pool.Exec(ctx, `
		UPDATE directories
		SET parent_id = $2, name = $3, updated_at = now()
		WHERE id = $1
	`, directory.ID(), parentID, directory.Name())
	if isUniqueViolation(err) {
		return ErrDirectoryNameConflict
	}
	if isForeignKeyViolation(err) {
		return ErrDirectoryParentInvalid
	}
	if err != nil {
		return fmt.Errorf("update directory: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrDirectoryNotFound
	}
	return nil
}

func (repository *PostgresRepository) WouldCreateDirectoryCycle(ctx context.Context, directoryID, newParentID string) (bool, error) {
	if newParentID == "" {
		return false, nil
	}
	var wouldCycle bool
	err := repository.pool.QueryRow(ctx, `
		WITH RECURSIVE ancestors AS (
			SELECT id, parent_id
			FROM directories
			WHERE id = $2
			UNION ALL
			SELECT parent.id, parent.parent_id
			FROM directories parent
			JOIN ancestors child ON parent.id = child.parent_id
		)
		SELECT EXISTS (SELECT 1 FROM ancestors WHERE id = $1)
	`, directoryID, newParentID).Scan(&wouldCycle)
	if err != nil {
		return false, fmt.Errorf("check directory cycle: %w", err)
	}
	return wouldCycle, nil
}

func (repository *PostgresRepository) CreateTag(ctx context.Context, tag *Tag) error {
	_, err := repository.pool.Exec(ctx, `INSERT INTO tags (id, name) VALUES ($1, $2)`, tag.ID(), tag.Name())
	if isUniqueViolation(err) {
		return ErrTagNameConflict
	}
	if err != nil {
		return fmt.Errorf("insert tag: %w", err)
	}
	return nil
}

func (repository *PostgresRepository) ListTags(ctx context.Context, limit, offset int) ([]*Tag, int, error) {
	var total int
	if err := repository.pool.QueryRow(ctx, `SELECT count(*) FROM tags`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count tags: %w", err)
	}
	rows, err := repository.pool.Query(ctx, `
		SELECT id::text, name
		FROM tags
		ORDER BY lower(name), id
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("query tags: %w", err)
	}
	defer rows.Close()

	tags := make([]*Tag, 0)
	for rows.Next() {
		tag, err := scanTag(rows)
		if err != nil {
			return nil, 0, err
		}
		tags = append(tags, tag)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate tags: %w", err)
	}
	return tags, total, nil
}

func (repository *PostgresRepository) GetTag(ctx context.Context, id string) (*Tag, error) {
	tag, err := scanTag(repository.pool.QueryRow(ctx, `SELECT id::text, name FROM tags WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrTagNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query tag: %w", err)
	}
	return tag, nil
}

func (repository *PostgresRepository) UpdateTag(ctx context.Context, tag *Tag) error {
	result, err := repository.pool.Exec(ctx, `
		UPDATE tags SET name = $2, updated_at = now() WHERE id = $1
	`, tag.ID(), tag.Name())
	if isUniqueViolation(err) {
		return ErrTagNameConflict
	}
	if err != nil {
		return fmt.Errorf("update tag: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrTagNotFound
	}
	return nil
}

func (repository *PostgresRepository) SetNoteTags(ctx context.Context, noteID string, tagIDs []string) error {
	transaction, err := repository.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin Note Tag update: %w", err)
	}
	defer func() { _ = transaction.Rollback(context.Background()) }()
	var noteExists bool
	if err := transaction.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM learning_notes WHERE id = $1 AND trashed_at IS NULL)`, noteID).Scan(&noteExists); err != nil {
		return fmt.Errorf("check Learning Note for Tags: %w", err)
	}
	if !noteExists {
		return ErrTaggableNoteNotFound
	}
	if len(tagIDs) > 0 {
		var existingTags int
		if err := transaction.QueryRow(ctx, `SELECT count(*) FROM tags WHERE id = ANY($1::uuid[])`, tagIDs).Scan(&existingTags); err != nil {
			return fmt.Errorf("validate Note Tags: %w", err)
		}
		if existingTags != len(tagIDs) {
			return ErrTagNotFound
		}
	}
	if _, err := transaction.Exec(ctx, `DELETE FROM note_tags WHERE note_id = $1`, noteID); err != nil {
		return fmt.Errorf("clear Note Tags: %w", err)
	}
	for _, tagID := range tagIDs {
		if _, err := transaction.Exec(ctx, `INSERT INTO note_tags (note_id, tag_id) VALUES ($1, $2)`, noteID, tagID); err != nil {
			return fmt.Errorf("attach Tag to Learning Note: %w", err)
		}
	}
	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit Note Tag update: %w", err)
	}
	return nil
}

func (repository *PostgresRepository) ListNoteTags(ctx context.Context, noteID string) ([]*Tag, error) {
	var noteExists bool
	if err := repository.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM learning_notes WHERE id = $1 AND trashed_at IS NULL)`, noteID).Scan(&noteExists); err != nil {
		return nil, fmt.Errorf("check Learning Note for Tags: %w", err)
	}
	if !noteExists {
		return nil, ErrTaggableNoteNotFound
	}
	rows, err := repository.pool.Query(ctx, `
		SELECT tags.id::text, tags.name
		FROM note_tags
		JOIN tags ON tags.id = note_tags.tag_id
		WHERE note_tags.note_id = $1
		ORDER BY lower(tags.name), tags.id
	`, noteID)
	if err != nil {
		return nil, fmt.Errorf("query Note Tags: %w", err)
	}
	defer rows.Close()
	tags := make([]*Tag, 0)
	for rows.Next() {
		tag, err := scanTag(rows)
		if err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate Note Tags: %w", err)
	}
	return tags, nil
}

func (repository *PostgresRepository) MergeTag(ctx context.Context, sourceID, targetID string) (*Tag, error) {
	if sourceID == targetID {
		return nil, ErrTagMergeSame
	}
	transaction, err := repository.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin Tag merge: %w", err)
	}
	defer func() { _ = transaction.Rollback(context.Background()) }()
	if _, err := scanTag(transaction.QueryRow(ctx, `SELECT id::text, name FROM tags WHERE id = $1 FOR UPDATE`, sourceID)); errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrTagNotFound
	} else if err != nil {
		return nil, fmt.Errorf("lock source Tag: %w", err)
	}
	target, err := scanTag(transaction.QueryRow(ctx, `SELECT id::text, name FROM tags WHERE id = $1 FOR UPDATE`, targetID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrTagNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock target Tag: %w", err)
	}
	if _, err := transaction.Exec(ctx, `
		INSERT INTO note_tags (note_id, tag_id)
		SELECT note_id, $2 FROM note_tags WHERE tag_id = $1
		ON CONFLICT DO NOTHING
	`, sourceID, targetID); err != nil {
		return nil, fmt.Errorf("move Note Tag associations: %w", err)
	}
	if _, err := transaction.Exec(ctx, `DELETE FROM tags WHERE id = $1`, sourceID); err != nil {
		return nil, fmt.Errorf("delete merged source Tag: %w", err)
	}
	if err := transaction.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit Tag merge: %w", err)
	}
	return target, nil
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

func scanDirectory(row rowScanner) (*Directory, error) {
	var id string
	var spaceID string
	var parentID string
	var name string
	if err := row.Scan(&id, &spaceID, &parentID, &name); err != nil {
		return nil, err
	}
	directory, err := NewDirectory(id, spaceID, parentID, name)
	if err != nil {
		return nil, fmt.Errorf("reconstruct directory: %w", err)
	}
	return directory, nil
}

func scanTag(row rowScanner) (*Tag, error) {
	var id string
	var name string
	if err := row.Scan(&id, &name); err != nil {
		return nil, err
	}
	tag, err := NewTag(id, name)
	if err != nil {
		return nil, fmt.Errorf("reconstruct tag: %w", err)
	}
	return tag, nil
}

func isUniqueViolation(err error) bool {
	var postgresError *pgconn.PgError
	return errors.As(err, &postgresError) && postgresError.Code == "23505"
}

func isForeignKeyViolation(err error) bool {
	var postgresError *pgconn.PgError
	return errors.As(err, &postgresError) && postgresError.Code == "23503"
}
