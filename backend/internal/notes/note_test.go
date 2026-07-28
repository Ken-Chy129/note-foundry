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

func TestRestoreMakesRevisionCurrentAndPreservesPublishedContent(t *testing.T) {
	note, _ := NewNote("note-1", "space-1", "", "Current", "current markdown")
	publishedAt := time.Date(2026, 7, 28, 10, 0, 0, 0, time.UTC)
	if _, err := note.Publish("publish-revision", publishedAt); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	target := Revision{
		ID:       "old-revision",
		NoteID:   note.ID(),
		Title:    "Old title",
		Slug:     "old-title",
		Markdown: "old markdown",
	}
	restoredAt := time.Date(2026, 7, 28, 11, 0, 0, 0, time.UTC)
	checkpoint, err := note.Restore(1, target, "restore-checkpoint", restoredAt)
	if err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	if checkpoint.Title != "Current" || checkpoint.Markdown != "current markdown" || checkpoint.Reason != RevisionReasonRestore {
		t.Errorf("restore checkpoint = %+v", checkpoint)
	}
	if note.Title() != "Old title" || note.Markdown() != "old markdown" || note.Version() != 2 {
		t.Errorf("restored note = title %q markdown %q version %d", note.Title(), note.Markdown(), note.Version())
	}
	if note.Published() == nil || note.Published().Title != "Current" || !note.Published().PublishedAt.Equal(publishedAt) {
		t.Errorf("Published Content changed during restore: %+v", note.Published())
	}
}

func TestManualCheckpointSnapshotsCurrentDraft(t *testing.T) {
	note, _ := NewNote("note-1", "space-1", "", "Current", "current markdown")
	now := time.Date(2026, 7, 28, 11, 0, 0, 0, time.UTC)
	revision, err := note.Checkpoint(1, "manual-revision", RevisionReasonManual, now)
	if err != nil {
		t.Fatalf("Checkpoint() error = %v", err)
	}
	if revision.Title != note.Title() || revision.Markdown != note.Markdown() || revision.Reason != RevisionReasonManual {
		t.Errorf("revision = %+v", revision)
	}
	if note.Version() != 1 {
		t.Errorf("version = %d, want unchanged", note.Version())
	}
}

func TestRelocatePreservesIdentityAndPrivateMoveClearsPublishedContent(t *testing.T) {
	note, _ := NewNote("note-1", "space-1", "directory-1", "Current", "markdown")
	if _, err := note.Publish("revision-1", time.Now()); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if err := note.Relocate("space-2", "directory-2"); err != nil {
		t.Fatalf("Relocate() error = %v", err)
	}
	note.MakePrivate()
	if note.ID() != "note-1" || note.SpaceID() != "space-2" || note.DirectoryID() != "directory-2" {
		t.Errorf("relocated identity/location = %q/%q/%q", note.ID(), note.SpaceID(), note.DirectoryID())
	}
	if note.Published() != nil {
		t.Errorf("Published Content = %+v, want nil", note.Published())
	}
}
