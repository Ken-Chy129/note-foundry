package attachments

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Ken-Chy129/note-foundry/backend/internal/platform/database"
)

func TestAttachmentRemainsPrivateUntilReferencedPublishedContent(t *testing.T) {
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
		INSERT INTO knowledge_spaces (id, name, visibility)
		VALUES ('11111111-1111-4111-8111-111111111111', 'AI Agent', 'public');
		INSERT INTO learning_notes (
			id, space_id, title, slug, current_markdown,
			published_title, published_slug, published_markdown, published_at
		) VALUES (
			'22222222-2222-4222-8222-222222222222',
			'11111111-1111-4111-8111-111111111111',
			'Diagram draft', 'diagram-draft', 'draft',
			'Diagram', 'diagram', 'published', now()
		);
	`); err != nil {
		t.Fatalf("insert Attachment fixtures: %v", err)
	}
	repository := NewPostgresRepository(pool)
	storage, err := NewLocalStorage(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalStorage() error = %v", err)
	}
	ids := []string{"33333333-3333-4333-8333-333333333333", "44444444-4444-4444-8444-444444444444"}
	index := 0
	service := NewService(repository, storage, func() string {
		id := ids[index]
		index++
		return id
	}, time.Now)
	attachment, err := service.Upload(ctx, "22222222-2222-4222-8222-222222222222", "diagram.svg", strings.NewReader("<svg></svg>"))
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	if _, _, err := service.OpenPublic(ctx, attachment.ID); !errors.Is(err, ErrAttachmentNotFound) {
		t.Fatalf("OpenPublic(draft Attachment) error = %v, want %v", err, ErrAttachmentNotFound)
	}
	publishedAt := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	if err := service.ReplacePublishedNoteAttachments(ctx, attachment.NoteID, []string{attachment.ID}, publishedAt); err != nil {
		t.Fatalf("ReplacePublishedNoteAttachments() error = %v", err)
	}
	publicAttachment, content, err := service.OpenPublic(ctx, attachment.ID)
	if err != nil {
		t.Fatalf("OpenPublic(published Attachment) error = %v", err)
	}
	content.Close()
	if publicAttachment.PublishedAt == nil || !publicAttachment.PublishedAt.Equal(publishedAt) {
		t.Errorf("public Attachment = %+v", publicAttachment)
	}
	if _, err := pool.Exec(ctx, `UPDATE knowledge_spaces SET visibility = 'private' WHERE id = $1`, "11111111-1111-4111-8111-111111111111"); err != nil {
		t.Fatalf("make Knowledge Space private: %v", err)
	}
	if _, _, err := service.OpenPublic(ctx, attachment.ID); !errors.Is(err, ErrAttachmentNotFound) {
		t.Fatalf("OpenPublic(private-space Attachment) error = %v, want %v", err, ErrAttachmentNotFound)
	}
}
