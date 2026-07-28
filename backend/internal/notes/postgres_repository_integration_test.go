package notes

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/Ken-Chy129/note-foundry/backend/internal/knowledge"
	"github.com/Ken-Chy129/note-foundry/backend/internal/platform/database"
)

func TestPostgresRepositoryPersistsDraftPublishAndRevisionLifecycle(t *testing.T) {
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

	knowledgeRepository := knowledge.NewPostgresRepository(pool)
	space, _ := knowledge.NewSpace("11111111-1111-4111-8111-111111111111", "AI Agent", knowledge.VisibilityPublic)
	if err := knowledgeRepository.CreateSpace(ctx, space); err != nil {
		t.Fatalf("CreateSpace() error = %v", err)
	}
	directory, _ := knowledge.NewDirectory("22222222-2222-4222-8222-222222222222", space.ID(), "", "Hermes Agent")
	if err := knowledgeRepository.CreateDirectory(ctx, directory); err != nil {
		t.Fatalf("CreateDirectory() error = %v", err)
	}

	repository := NewPostgresRepository(pool)
	note, _ := NewNote(
		"33333333-3333-4333-8333-333333333333",
		space.ID(),
		directory.ID(),
		"Agent Loop",
		"first draft",
	)
	if err := repository.CreateNote(ctx, note); err != nil {
		t.Fatalf("CreateNote() error = %v", err)
	}
	firstCopy, err := repository.GetNote(ctx, note.ID())
	if err != nil {
		t.Fatalf("first GetNote() error = %v", err)
	}
	staleCopy, err := repository.GetNote(ctx, note.ID())
	if err != nil {
		t.Fatalf("second GetNote() error = %v", err)
	}

	if err := firstCopy.Autosave(1, "Agent Loop", "complete draft"); err != nil {
		t.Fatalf("Autosave() error = %v", err)
	}
	if err := repository.UpdateDraft(ctx, firstCopy, 1); err != nil {
		t.Fatalf("UpdateDraft() error = %v", err)
	}
	if err := staleCopy.Autosave(1, "Agent Loop stale", "stale draft"); err != nil {
		t.Fatalf("stale local Autosave() error = %v", err)
	}
	if err := repository.UpdateDraft(ctx, staleCopy, 1); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("stale UpdateDraft() error = %v, want %v", err, ErrVersionConflict)
	}

	publishAt := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	revision, err := firstCopy.Publish("44444444-4444-4444-8444-444444444444", publishAt)
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if err := repository.Publish(ctx, firstCopy, revision); err != nil {
		t.Fatalf("repository.Publish() error = %v", err)
	}

	published, err := repository.GetNote(ctx, note.ID())
	if err != nil {
		t.Fatalf("published GetNote() error = %v", err)
	}
	if published.Published() == nil || published.Published().Markdown != "complete draft" {
		t.Errorf("Published Content = %+v", published.Published())
	}
	var revisionCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM note_revisions WHERE note_id = $1`, note.ID()).Scan(&revisionCount); err != nil {
		t.Fatalf("count revisions: %v", err)
	}
	if revisionCount != 1 {
		t.Errorf("revision count = %d, want 1", revisionCount)
	}

	manualRevision, err := firstCopy.Checkpoint(2, "55555555-5555-4555-8555-555555555555", RevisionReasonManual, publishAt.Add(time.Minute))
	if err != nil {
		t.Fatalf("Checkpoint() error = %v", err)
	}
	if err := repository.CreateCheckpoint(ctx, firstCopy.ID(), 2, manualRevision); err != nil {
		t.Fatalf("CreateCheckpoint() error = %v", err)
	}
	if err := firstCopy.Autosave(2, "Agent Loop revised", "regressed draft"); err != nil {
		t.Fatalf("second Autosave() error = %v", err)
	}
	if err := repository.UpdateDraft(ctx, firstCopy, 2); err != nil {
		t.Fatalf("second UpdateDraft() error = %v", err)
	}
	target, err := repository.GetRevision(ctx, firstCopy.ID(), revision.ID)
	if err != nil {
		t.Fatalf("GetRevision() error = %v", err)
	}
	checkpoint, err := firstCopy.Restore(3, target, "66666666-6666-4666-8666-666666666666", publishAt.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	if err := repository.Restore(ctx, firstCopy, checkpoint, 3); err != nil {
		t.Fatalf("repository.Restore() error = %v", err)
	}
	restored, err := repository.GetNote(ctx, firstCopy.ID())
	if err != nil {
		t.Fatalf("restored GetNote() error = %v", err)
	}
	if restored.Markdown() != "complete draft" || restored.Version() != 4 {
		t.Errorf("restored note = markdown %q version %d", restored.Markdown(), restored.Version())
	}
	if restored.Published() == nil || restored.Published().Markdown != "complete draft" {
		t.Errorf("Published Content changed during restore: %+v", restored.Published())
	}
	page, err := repository.ListRevisions(ctx, firstCopy.ID(), 1, 20)
	if err != nil {
		t.Fatalf("ListRevisions() error = %v", err)
	}
	if page.TotalItems != 3 || len(page.Revisions) != 3 || page.Revisions[0].Reason != RevisionReasonRestore {
		t.Errorf("revision page = %+v", page)
	}
	ownerPage, err := repository.ListNotes(ctx, NoteListFilter{SpaceID: space.ID(), Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("ListNotes() error = %v", err)
	}
	if ownerPage.TotalItems != 1 || len(ownerPage.Notes) != 1 || ownerPage.Notes[0].ID() != note.ID() {
		t.Errorf("owner note page = %+v", ownerPage)
	}
	publicNote, err := repository.GetPublishedNote(ctx, note.ID())
	if err != nil {
		t.Fatalf("GetPublishedNote() error = %v", err)
	}
	if publicNote.Markdown != "complete draft" || publicNote.Title != "Agent Loop" {
		t.Errorf("public note = %+v", publicNote)
	}
	publicPage, err := repository.ListPublishedNotes(ctx, space.ID(), 1, 20)
	if err != nil {
		t.Fatalf("ListPublishedNotes() error = %v", err)
	}
	if publicPage.TotalItems != 1 || len(publicPage.Notes) != 1 {
		t.Errorf("public note page = %+v", publicPage)
	}
}
