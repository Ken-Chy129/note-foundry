package search

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Ken-Chy129/note-foundry/backend/internal/platform/database"
)

func TestPostgresSearchFindsEnglishAndChineseWithoutExposingDraftsOrPrivateNotes(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool, err := database.Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("database.Open() error = %v", err)
	}
	defer pool.Close()
	if err := database.ApplyMigrations(ctx, pool); err != nil {
		t.Fatalf("ApplyMigrations() error = %v", err)
	}
	if _, err := pool.Exec(ctx, `TRUNCATE knowledge_spaces CASCADE`); err != nil {
		t.Fatalf("truncate knowledge data: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO knowledge_spaces (id, name, visibility) VALUES
			('11111111-1111-4111-8111-111111111111', 'AI Agent', 'public'),
			('22222222-2222-4222-8222-222222222222', 'Private', 'private');
		INSERT INTO learning_notes (
			id, space_id, title, slug, current_markdown,
			published_title, published_slug, published_markdown, published_at
		) VALUES
			('33333333-3333-4333-8333-333333333333', '11111111-1111-4111-8111-111111111111', 'Hermes Agent Architecture draft', 'hermes-agent-architecture-draft', '# 上下文管理\n未完成内容', 'Hermes Agent Architecture', 'hermes-agent-architecture', '# 记忆设计\n公开的记忆管理内容', now()),
			('44444444-4444-4444-8444-444444444444', '22222222-2222-4222-8222-222222222222', 'Secret Memory', 'secret-memory', 'private memory design', 'Secret Memory', 'secret-memory', 'private memory design', now());
	`); err != nil {
		t.Fatalf("insert search fixtures: %v", err)
	}
	repository := NewPostgresRepository(pool)
	service := NewService(repository)
	if err := service.ProjectCurrentNote(ctx, "33333333-3333-4333-8333-333333333333", "Hermes Agent Architecture draft", "# 上下文管理\n未完成内容"); err != nil {
		t.Fatalf("ProjectCurrentNote(public) error = %v", err)
	}
	if err := service.ProjectPublishedNote(ctx, "33333333-3333-4333-8333-333333333333", "Hermes Agent Architecture", "# 记忆设计\n公开的记忆管理内容"); err != nil {
		t.Fatalf("ProjectPublishedNote(public) error = %v", err)
	}
	if err := service.ProjectCurrentNote(ctx, "44444444-4444-4444-8444-444444444444", "Secret Memory", "private memory design"); err != nil {
		t.Fatalf("ProjectCurrentNote(private) error = %v", err)
	}
	if err := service.ProjectPublishedNote(ctx, "44444444-4444-4444-8444-444444444444", "Secret Memory", "private memory design"); err != nil {
		t.Fatalf("ProjectPublishedNote(private) error = %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO tags (id, name) VALUES ('55555555-5555-4555-8555-555555555555', 'agent-memory');
		INSERT INTO note_tags (note_id, tag_id) VALUES ('33333333-3333-4333-8333-333333333333', '55555555-5555-4555-8555-555555555555');
	`); err != nil {
		t.Fatalf("insert search Tag fixture: %v", err)
	}
	if err := service.RefreshNoteTags(ctx, "33333333-3333-4333-8333-333333333333"); err != nil {
		t.Fatalf("RefreshNoteTags() error = %v", err)
	}

	ownerChinese, err := service.SearchOwner(ctx, Options{Query: "上下文", Page: 1, PageSize: 20})
	if err != nil || ownerChinese.TotalItems != 1 {
		t.Fatalf("owner Chinese search = %+v, %v", ownerChinese, err)
	}
	publicDraft, err := service.SearchPublic(ctx, Options{Query: "上下文", Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("public draft search error = %v", err)
	}
	if publicDraft.TotalItems != 0 {
		t.Errorf("public draft search leaked results: %+v", publicDraft)
	}
	publicChinese, err := service.SearchPublic(ctx, Options{Query: "记忆", Page: 1, PageSize: 20})
	if err != nil || publicChinese.TotalItems != 1 || publicChinese.Results[0].ID != "33333333-3333-4333-8333-333333333333" {
		t.Fatalf("public Chinese search = %+v, %v", publicChinese, err)
	}
	publicEnglish, err := service.SearchPublic(ctx, Options{Query: "architecture", Page: 1, PageSize: 20})
	if err != nil || publicEnglish.TotalItems != 1 {
		t.Fatalf("public English search = %+v, %v", publicEnglish, err)
	}
	publicTag, err := service.SearchPublic(ctx, Options{Query: "agent-memory", TagID: "55555555-5555-4555-8555-555555555555", Page: 1, PageSize: 20})
	if err != nil || publicTag.TotalItems != 1 {
		t.Fatalf("public Tag search = %+v, %v", publicTag, err)
	}
	publicPrivate, err := service.SearchPublic(ctx, Options{Query: "secret memory", Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("public private-note search error = %v", err)
	}
	if publicPrivate.TotalItems != 0 {
		t.Errorf("private note leaked through public search: %+v", publicPrivate)
	}
}
