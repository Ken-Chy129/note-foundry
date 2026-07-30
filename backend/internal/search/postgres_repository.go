package search

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (repository *PostgresRepository) UpsertCurrentNote(ctx context.Context, noteID, title, headings, body string) error {
	_, err := repository.pool.Exec(ctx, `
		INSERT INTO note_search_documents (
			note_id, current_title, current_headings, current_body, current_tags
		)
		VALUES (
			$1, $2, $3, $4,
			COALESCE((
				SELECT string_agg(tags.name, ' ' ORDER BY lower(tags.name), tags.id)
				FROM note_tags JOIN tags ON tags.id = note_tags.tag_id
				WHERE note_tags.note_id = $1
			), '')
		)
		ON CONFLICT (note_id) DO UPDATE
		SET current_title = EXCLUDED.current_title,
			current_headings = EXCLUDED.current_headings,
			current_body = EXCLUDED.current_body,
			current_tags = EXCLUDED.current_tags,
			updated_at = now()
	`, noteID, title, headings, body)
	if err != nil {
		return fmt.Errorf("upsert current search document: %w", err)
	}
	return nil
}

func (repository *PostgresRepository) UpsertPublishedNote(ctx context.Context, noteID, title, headings, body string) error {
	result, err := repository.pool.Exec(ctx, `
		INSERT INTO note_search_documents (
			note_id, current_title, current_headings, current_body, current_tags,
			published_available, published_title, published_headings, published_body, published_tags
		)
		SELECT learning_notes.id,
			learning_notes.title,
			'',
			learning_notes.current_markdown,
			COALESCE(tag_names.names, ''),
			true,
			$2,
			$3,
			$4,
			COALESCE(tag_names.names, '')
		FROM learning_notes
		LEFT JOIN LATERAL (
			SELECT string_agg(tags.name, ' ' ORDER BY lower(tags.name), tags.id) AS names
			FROM note_tags JOIN tags ON tags.id = note_tags.tag_id
			WHERE note_tags.note_id = learning_notes.id
		) tag_names ON true
		WHERE learning_notes.id = $1
		ON CONFLICT (note_id) DO UPDATE
		SET published_available = true,
			published_title = EXCLUDED.published_title,
			published_headings = EXCLUDED.published_headings,
			published_body = EXCLUDED.published_body,
			published_tags = EXCLUDED.published_tags,
			updated_at = now()
	`, noteID, title, headings, body)
	if err != nil {
		return fmt.Errorf("upsert published search document: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("Learning Note not found for published search projection")
	}
	return nil
}

func (repository *PostgresRepository) RefreshNoteTags(ctx context.Context, noteID string) error {
	_, err := repository.pool.Exec(ctx, `
		UPDATE note_search_documents
		SET current_tags = COALESCE((
				SELECT string_agg(tags.name, ' ' ORDER BY lower(tags.name), tags.id)
				FROM note_tags JOIN tags ON tags.id = note_tags.tag_id
				WHERE note_tags.note_id = $1
			), ''),
			published_tags = COALESCE((
				SELECT string_agg(tags.name, ' ' ORDER BY lower(tags.name), tags.id)
				FROM note_tags JOIN tags ON tags.id = note_tags.tag_id
				WHERE note_tags.note_id = $1
			), ''),
			updated_at = now()
		WHERE note_id = $1
	`, noteID)
	if err != nil {
		return fmt.Errorf("refresh Note Tag search text: %w", err)
	}
	return nil
}

func (repository *PostgresRepository) RefreshNotesForTag(ctx context.Context, tagID string) error {
	_, err := repository.pool.Exec(ctx, `
		UPDATE note_search_documents document
		SET current_tags = COALESCE((
				SELECT string_agg(tags.name, ' ' ORDER BY lower(tags.name), tags.id)
				FROM note_tags JOIN tags ON tags.id = note_tags.tag_id
				WHERE note_tags.note_id = document.note_id
			), ''),
			published_tags = COALESCE((
				SELECT string_agg(tags.name, ' ' ORDER BY lower(tags.name), tags.id)
				FROM note_tags JOIN tags ON tags.id = note_tags.tag_id
				WHERE note_tags.note_id = document.note_id
			), ''),
			updated_at = now()
		WHERE document.note_id IN (SELECT note_id FROM note_tags WHERE tag_id = $1)
	`, tagID)
	if err != nil {
		return fmt.Errorf("refresh search documents for Tag: %w", err)
	}
	return nil
}

func (repository *PostgresRepository) SearchOwner(ctx context.Context, options Options) (Page, error) {
	return repository.search(ctx, options, false)
}

func (repository *PostgresRepository) SearchPublic(ctx context.Context, options Options) (Page, error) {
	return repository.search(ctx, options, true)
}

func (repository *PostgresRepository) search(ctx context.Context, options Options, public bool) (Page, error) {
	spaceID := optionalString(options.SpaceID)
	tagID := optionalString(options.TagID)
	visibilityJoin := ""
	visibilityPredicate := ""
	titleColumn := "document.current_title"
	slugColumn := "learning_notes.slug"
	bodyColumn := "document.current_body"
	vectorColumn := "document.current_vector"
	textColumn := "document.current_text"
	updatedAtColumn := "learning_notes.updated_at"
	if public {
		visibilityJoin = "JOIN knowledge_spaces ON knowledge_spaces.id = learning_notes.space_id"
		visibilityPredicate = "AND knowledge_spaces.visibility = 'public' AND learning_notes.published_at IS NOT NULL AND document.published_available"
		titleColumn = "document.published_title"
		slugColumn = "learning_notes.published_slug"
		bodyColumn = "document.published_body"
		vectorColumn = "document.published_vector"
		textColumn = "document.published_text"
		updatedAtColumn = "learning_notes.published_at"
	}
	predicate := fmt.Sprintf(`
		FROM note_search_documents document
		JOIN learning_notes ON learning_notes.id = document.note_id
		%s
		WHERE learning_notes.trashed_at IS NULL
			%s
			AND ($2::uuid IS NULL OR learning_notes.space_id = $2)
			AND ($3::uuid IS NULL OR EXISTS (
				SELECT 1 FROM note_tags WHERE note_tags.note_id = document.note_id AND note_tags.tag_id = $3
			))
			AND (
				%s @@ websearch_to_tsquery('simple'::regconfig, $1)
				OR %s ILIKE '%%' || $1 || '%%'
				OR similarity(%s, $1) >= 0.25
			)
	`, visibilityJoin, visibilityPredicate, vectorColumn, textColumn, textColumn)
	var totalItems int
	if err := repository.pool.QueryRow(ctx, `SELECT count(*) `+predicate, options.Query, spaceID, tagID).Scan(&totalItems); err != nil {
		return Page{}, fmt.Errorf("count search results: %w", err)
	}
	query := fmt.Sprintf(`
		SELECT document.note_id::text,
			learning_notes.space_id::text,
			%s,
			%s,
			left(%s, 320),
			%s,
			(
				ts_rank_cd(%s, websearch_to_tsquery('simple'::regconfig, $1)) * 4
				+ similarity(%s, $1)
				+ CASE WHEN %s ILIKE '%%' || $1 || '%%' THEN 1 ELSE 0 END
			)::float8 AS rank
		%s
		ORDER BY rank DESC, lower(%s), document.note_id
		LIMIT $4 OFFSET $5
	`, titleColumn, slugColumn, bodyColumn, updatedAtColumn, vectorColumn, textColumn, textColumn, predicate, titleColumn)
	rows, err := repository.pool.Query(ctx, query, options.Query, spaceID, tagID, options.PageSize, (options.Page-1)*options.PageSize)
	if err != nil {
		return Page{}, fmt.Errorf("query search results: %w", err)
	}
	defer rows.Close()
	results := make([]Result, 0, options.PageSize)
	for rows.Next() {
		var result Result
		if err := rows.Scan(&result.ID, &result.SpaceID, &result.Title, &result.Slug, &result.Snippet, &result.UpdatedAt, &result.Rank); err != nil {
			return Page{}, fmt.Errorf("scan search result: %w", err)
		}
		results = append(results, result)
	}
	if err := rows.Err(); err != nil {
		return Page{}, fmt.Errorf("iterate search results: %w", err)
	}
	return Page{Results: results, Page: options.Page, PageSize: options.PageSize, TotalItems: totalItems}, nil
}

func optionalString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
