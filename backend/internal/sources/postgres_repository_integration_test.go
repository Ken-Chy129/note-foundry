package sources

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/Ken-Chy129/note-foundry/backend/internal/platform/database"
)

func TestPostgresRepositoryPersistsManualSourceInbox(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := database.Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("database.Open() error = %v", err)
	}
	defer pool.Close()
	if err := database.ApplyMigrations(ctx, pool); err != nil {
		t.Fatalf("ApplyMigrations() error = %v", err)
	}
	if _, err := pool.Exec(ctx, `TRUNCATE learning_sources CASCADE`); err != nil {
		t.Fatalf("truncate Learning Sources: %v", err)
	}

	repository := NewPostgresRepository(pool)
	createdAt := time.Date(2026, 7, 30, 15, 0, 0, 0, time.UTC)
	source, _ := NewManualSource(
		"11111111-1111-4111-8111-111111111111",
		"PostgreSQL query planning",
		"Compare query plans",
		"EXPLAIN ANALYZE output",
		createdAt,
	)
	if err := repository.CreateSource(ctx, source); err != nil {
		t.Fatalf("CreateSource() error = %v", err)
	}

	stored, err := repository.GetSource(ctx, source.ID())
	if err != nil {
		t.Fatalf("GetSource() error = %v", err)
	}
	if stored.Title() != source.Title() || stored.CaptureNote() != source.CaptureNote() || stored.Content() != source.Content() {
		t.Errorf("stored source = title:%q capture:%q content:%q", stored.Title(), stored.CaptureNote(), stored.Content())
	}

	page, err := repository.ListSources(ctx, ListFilter{InboxOnly: true, Page: 1, PageSize: 50})
	if err != nil {
		t.Fatalf("ListSources() error = %v", err)
	}
	if page.TotalItems != 1 || len(page.Sources) != 1 || page.Sources[0].ID != source.ID() {
		t.Fatalf("Source Inbox page = %+v", page)
	}
	if page.Sources[0].SpaceID != "" || page.Sources[0].ProcessingStatus != ProcessingStatusReady {
		t.Errorf("Source Inbox summary = %+v", page.Sources[0])
	}

	spaceID := "22222222-2222-4222-8222-222222222222"
	if _, err := pool.Exec(ctx, `
		INSERT INTO knowledge_spaces (id, name, visibility)
		VALUES ($1, 'Source organization integration', 'private')
		ON CONFLICT DO NOTHING
	`, spaceID); err != nil {
		t.Fatalf("insert target Knowledge Space: %v", err)
	}
	source.Organize(spaceID, createdAt.Add(time.Hour))
	if err := repository.UpdateSourceOrganization(ctx, source); err != nil {
		t.Fatalf("UpdateSourceOrganization() error = %v", err)
	}

	spacePage, err := repository.ListSources(ctx, ListFilter{SpaceID: spaceID, Page: 1, PageSize: 50})
	if err != nil {
		t.Fatalf("ListSources(space) error = %v", err)
	}
	if spacePage.TotalItems != 1 || len(spacePage.Sources) != 1 || spacePage.Sources[0].SpaceID != spaceID {
		t.Fatalf("organized source page = %+v", spacePage)
	}
	inboxPage, err := repository.ListSources(ctx, ListFilter{InboxOnly: true, Page: 1, PageSize: 50})
	if err != nil {
		t.Fatalf("ListSources(inbox after organize) error = %v", err)
	}
	if inboxPage.TotalItems != 0 || len(inboxPage.Sources) != 0 {
		t.Fatalf("Source Inbox after organize = %+v", inboxPage)
	}

	urlSource, _ := NewURLSource(
		"33333333-3333-4333-8333-333333333333",
		"PostgreSQL documentation",
		"Read planner notes",
		"https://www.postgresql.org/docs/current/using-explain.html#EXAMPLE",
		"https://www.postgresql.org/docs/current/using-explain.html",
		createdAt,
	)
	if err := repository.CreateSource(ctx, urlSource); err != nil {
		t.Fatalf("CreateSource(URL) error = %v", err)
	}
	foundURL, err := repository.FindSourceByNormalizedURL(ctx, urlSource.NormalizedURL())
	if err != nil || foundURL.ID() != urlSource.ID() || foundURL.OriginalURL() != urlSource.OriginalURL() {
		t.Fatalf("FindSourceByNormalizedURL() = %+v, %v", foundURL, err)
	}
	duplicateURL, _ := NewURLSource(
		"44444444-4444-4444-8444-444444444444",
		"Duplicate",
		"",
		urlSource.NormalizedURL(),
		urlSource.NormalizedURL(),
		createdAt,
	)
	if err := repository.CreateSource(ctx, duplicateURL); !errors.Is(err, ErrSourceURLConflict) {
		t.Fatalf("duplicate URL error = %v, want %v", err, ErrSourceURLConflict)
	}
}
