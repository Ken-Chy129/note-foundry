package notes

import (
	"errors"
	"testing"
	"time"
)

func TestNewNoteCreatesStableMarkdownKnowledgeUnit(t *testing.T) {
	note, err := NewNote(
		"11111111-1111-4111-8111-111111111111",
		"22222222-2222-4222-8222-222222222222",
		"33333333-3333-4333-8333-333333333333",
		"  Memory Manager 如何工作  ",
		"# Memory\n\nCanonical Markdown.",
	)
	if err != nil {
		t.Fatalf("NewNote() error = %v", err)
	}

	if note.ID() != "11111111-1111-4111-8111-111111111111" || note.SpaceID() != "22222222-2222-4222-8222-222222222222" {
		t.Errorf("identity = id:%q space:%q", note.ID(), note.SpaceID())
	}
	if note.Title() != "Memory Manager 如何工作" || note.Slug() != "memory-manager-如何工作" {
		t.Errorf("title = %q, slug = %q", note.Title(), note.Slug())
	}
	if note.Version() != 1 || note.Markdown() != "# Memory\n\nCanonical Markdown." {
		t.Errorf("version = %d, markdown = %q", note.Version(), note.Markdown())
	}
	if note.Published() != nil {
		t.Error("new note unexpectedly has Published Content")
	}
}

func TestAutosaveUsesOptimisticVersionAndKeepsPublishedContentStable(t *testing.T) {
	note, _ := NewNote("11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222", "", "Agent Loop", "first")
	publishedAt := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	if _, err := note.Publish("44444444-4444-4444-8444-444444444444", publishedAt); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	if err := note.Autosave(1, "Agent Loop revised", "unfinished draft"); err != nil {
		t.Fatalf("Autosave() error = %v", err)
	}
	if note.Version() != 2 || note.Title() != "Agent Loop revised" || note.Markdown() != "unfinished draft" {
		t.Errorf("draft = version:%d title:%q markdown:%q", note.Version(), note.Title(), note.Markdown())
	}
	if note.Published().Title != "Agent Loop" || note.Published().Markdown != "first" {
		t.Errorf("Published Content changed during autosave: %+v", note.Published())
	}

	if err := note.Autosave(1, "stale", "stale"); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("stale Autosave() error = %v, want %v", err, ErrVersionConflict)
	}
}

func TestPublishCreatesRecoverableRevisionAndPublishedSnapshot(t *testing.T) {
	note, _ := NewNote("11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222", "", "Tools & Skills", "content")
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)

	revision, err := note.Publish("44444444-4444-4444-8444-444444444444", now)
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if revision.NoteID != note.ID() || revision.Reason != RevisionReasonPublish || revision.Markdown != "content" {
		t.Errorf("revision = %+v", revision)
	}
	if note.Published() == nil || note.Published().Markdown != "content" || !note.Published().PublishedAt.Equal(now) {
		t.Errorf("Published Content = %+v", note.Published())
	}
}
