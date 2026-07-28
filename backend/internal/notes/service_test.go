package notes

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Ken-Chy129/note-foundry/backend/internal/knowledge"
)

func TestServiceCreatesDraftInPublicKnowledgeSpace(t *testing.T) {
	space, _ := knowledge.NewSpace("11111111-1111-4111-8111-111111111111", "AI Agent", knowledge.VisibilityPublic)
	directory, _ := knowledge.NewDirectory("22222222-2222-4222-8222-222222222222", space.ID(), "", "Hermes Agent")
	repository := &noteRepositoryStub{}
	service := NewService(ServiceConfig{
		Notes:      repository,
		Knowledge:  &knowledgeCatalogStub{space: space, directory: directory},
		GenerateID: idSequence("33333333-3333-4333-8333-333333333333"),
		Now:        time.Now,
	})

	note, err := service.CreateNote(context.Background(), space.ID(), directory.ID(), "Architecture", "draft")
	if err != nil {
		t.Fatalf("CreateNote() error = %v", err)
	}
	if repository.created != note || note.Published() != nil {
		t.Errorf("created note = %+v", note)
	}
}

func TestServiceRejectsDirectoryFromAnotherKnowledgeSpace(t *testing.T) {
	space, _ := knowledge.NewSpace("11111111-1111-4111-8111-111111111111", "AI Agent", knowledge.VisibilityPublic)
	directory, _ := knowledge.NewDirectory("22222222-2222-4222-8222-222222222222", "99999999-9999-4999-8999-999999999999", "", "Other")
	service := NewService(ServiceConfig{
		Notes:      &noteRepositoryStub{},
		Knowledge:  &knowledgeCatalogStub{space: space, directory: directory},
		GenerateID: idSequence("33333333-3333-4333-8333-333333333333"),
		Now:        time.Now,
	})

	_, err := service.CreateNote(context.Background(), space.ID(), directory.ID(), "Architecture", "draft")
	if !errors.Is(err, ErrInvalidNoteDirectory) {
		t.Fatalf("CreateNote() error = %v, want %v", err, ErrInvalidNoteDirectory)
	}
}

func TestServiceAutosavesAndPublishesReviewedVersion(t *testing.T) {
	space, _ := knowledge.NewSpace("11111111-1111-4111-8111-111111111111", "AI Agent", knowledge.VisibilityPublic)
	note, _ := NewNote("33333333-3333-4333-8333-333333333333", space.ID(), "", "Agent Loop", "draft")
	repository := &noteRepositoryStub{found: note}
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	service := NewService(ServiceConfig{
		Notes:      repository,
		Knowledge:  &knowledgeCatalogStub{space: space},
		GenerateID: idSequence("44444444-4444-4444-8444-444444444444"),
		Now:        func() time.Time { return now },
	})

	updated, err := service.Autosave(context.Background(), note.ID(), 1, "Agent Loop", "complete")
	if err != nil {
		t.Fatalf("Autosave() error = %v", err)
	}
	if updated.Version() != 2 || repository.updatedExpectedVersion != 1 {
		t.Errorf("updated version = %d, expected persisted version = %d", updated.Version(), repository.updatedExpectedVersion)
	}
	published, err := service.Publish(context.Background(), note.ID(), 2)
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if published.Published() == nil || repository.publishedRevision.Reason != RevisionReasonPublish {
		t.Errorf("published note = %+v, revision = %+v", published.Published(), repository.publishedRevision)
	}
}

func TestServiceRejectsPublishingPrivateNote(t *testing.T) {
	space, _ := knowledge.NewSpace("11111111-1111-4111-8111-111111111111", "Private", knowledge.VisibilityPrivate)
	note, _ := NewNote("33333333-3333-4333-8333-333333333333", space.ID(), "", "Private Note", "content")
	service := NewService(ServiceConfig{
		Notes:      &noteRepositoryStub{found: note},
		Knowledge:  &knowledgeCatalogStub{space: space},
		GenerateID: idSequence("44444444-4444-4444-8444-444444444444"),
		Now:        time.Now,
	})

	_, err := service.Publish(context.Background(), note.ID(), 1)
	if !errors.Is(err, ErrPrivateNotePublish) {
		t.Fatalf("Publish() error = %v, want %v", err, ErrPrivateNotePublish)
	}
}

func TestServiceCreatesListsAndRestoresNoteRevisions(t *testing.T) {
	space, _ := knowledge.NewSpace("11111111-1111-4111-8111-111111111111", "AI Agent", knowledge.VisibilityPublic)
	note, _ := NewNote("33333333-3333-4333-8333-333333333333", space.ID(), "", "Current", "current draft")
	target := Revision{
		ID:        "44444444-4444-4444-8444-444444444444",
		NoteID:    note.ID(),
		Title:     "Earlier",
		Slug:      "earlier",
		Markdown:  "earlier draft",
		Reason:    RevisionReasonPublish,
		CreatedAt: time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC),
	}
	repository := &noteRepositoryStub{found: note, revision: target, revisionPage: RevisionPage{Revisions: []Revision{target}, Page: 1, PageSize: 20, TotalItems: 1}}
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	service := NewService(ServiceConfig{
		Notes:      repository,
		Knowledge:  &knowledgeCatalogStub{space: space},
		GenerateID: idSequence("55555555-5555-4555-8555-555555555555", "66666666-6666-4666-8666-666666666666"),
		Now:        func() time.Time { return now },
	})

	manual, err := service.CreateCheckpoint(context.Background(), note.ID(), 1)
	if err != nil {
		t.Fatalf("CreateCheckpoint() error = %v", err)
	}
	if manual.Reason != RevisionReasonManual || repository.checkpoint.ID != manual.ID {
		t.Errorf("manual checkpoint = %+v, persisted = %+v", manual, repository.checkpoint)
	}
	page, err := service.ListRevisions(context.Background(), note.ID(), 1, 20)
	if err != nil || page.TotalItems != 1 {
		t.Fatalf("ListRevisions() = %+v, %v", page, err)
	}
	restored, err := service.Restore(context.Background(), note.ID(), target.ID, 1)
	if err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	if restored.Markdown() != "earlier draft" || restored.Version() != 2 || repository.restoreCheckpoint.Reason != RevisionReasonRestore {
		t.Errorf("restored = %+v, checkpoint = %+v", restored, repository.restoreCheckpoint)
	}
}

type noteRepositoryStub struct {
	created                *Note
	found                  *Note
	updated                *Note
	updatedExpectedVersion int64
	published              *Note
	publishedRevision      Revision
	checkpoint             Revision
	revision               Revision
	revisionPage           RevisionPage
	restored               *Note
	restoreCheckpoint      Revision
}

func (repository *noteRepositoryStub) CreateNote(_ context.Context, note *Note) error {
	repository.created = note
	return nil
}

func (repository *noteRepositoryStub) GetNote(context.Context, string) (*Note, error) {
	return repository.found, nil
}

func (repository *noteRepositoryStub) UpdateDraft(_ context.Context, note *Note, expectedVersion int64) error {
	repository.updated = note
	repository.updatedExpectedVersion = expectedVersion
	return nil
}

func (repository *noteRepositoryStub) Publish(_ context.Context, note *Note, revision Revision) error {
	repository.published = note
	repository.publishedRevision = revision
	return nil
}

func (repository *noteRepositoryStub) CreateCheckpoint(_ context.Context, _ string, _ int64, revision Revision) error {
	repository.checkpoint = revision
	return nil
}

func (repository *noteRepositoryStub) ListRevisions(context.Context, string, int, int) (RevisionPage, error) {
	return repository.revisionPage, nil
}

func (repository *noteRepositoryStub) GetRevision(context.Context, string, string) (Revision, error) {
	return repository.revision, nil
}

func (repository *noteRepositoryStub) Restore(_ context.Context, note *Note, checkpoint Revision, _ int64) error {
	repository.restored = note
	repository.restoreCheckpoint = checkpoint
	return nil
}

type knowledgeCatalogStub struct {
	space     *knowledge.Space
	directory *knowledge.Directory
}

func (catalog *knowledgeCatalogStub) GetSpace(context.Context, string) (*knowledge.Space, error) {
	return catalog.space, nil
}

func (catalog *knowledgeCatalogStub) GetDirectory(context.Context, string) (*knowledge.Directory, error) {
	return catalog.directory, nil
}

func idSequence(ids ...string) IDGenerator {
	index := 0
	return func() string {
		id := ids[index]
		index++
		return id
	}
}
