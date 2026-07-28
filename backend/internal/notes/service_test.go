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

func TestServiceSeparatesOwnerDraftFromAnonymousPublishedContent(t *testing.T) {
	space, _ := knowledge.NewSpace("11111111-1111-4111-8111-111111111111", "AI Agent", knowledge.VisibilityPublic)
	note, _ := NewNote("33333333-3333-4333-8333-333333333333", space.ID(), "", "Draft title", "unfinished draft")
	published := PublishedNote{
		ID:       note.ID(),
		SpaceID:  space.ID(),
		Title:    "Published title",
		Slug:     "published-title",
		Markdown: "reviewed content",
	}
	repository := &noteRepositoryStub{
		notePage:          NotePage{Notes: []*Note{note}, Page: 1, PageSize: 20, TotalItems: 1},
		publishedNote:     published,
		publishedNotePage: PublishedNotePage{Notes: []PublishedNote{published}, Page: 1, PageSize: 20, TotalItems: 1},
	}
	service := NewService(ServiceConfig{Notes: repository, Knowledge: &knowledgeCatalogStub{space: space}, GenerateID: idSequence("unused"), Now: time.Now})

	ownerPage, err := service.ListNotes(context.Background(), NoteListFilter{SpaceID: space.ID(), Page: 1, PageSize: 20})
	if err != nil || ownerPage.Notes[0].Markdown() != "unfinished draft" {
		t.Fatalf("owner ListNotes() = %+v, %v", ownerPage, err)
	}
	publicNote, err := service.GetPublishedNote(context.Background(), note.ID())
	if err != nil || publicNote.Markdown != "reviewed content" {
		t.Fatalf("GetPublishedNote() = %+v, %v", publicNote, err)
	}
	publicPage, err := service.ListPublishedNotes(context.Background(), space.ID(), 1, 20)
	if err != nil || publicPage.TotalItems != 1 {
		t.Fatalf("ListPublishedNotes() = %+v, %v", publicPage, err)
	}
}

func TestServiceTrashRestoreRequiresPublicConfirmationAndPreservesIdentity(t *testing.T) {
	space, _ := knowledge.NewSpace("11111111-1111-4111-8111-111111111111", "AI Agent", knowledge.VisibilityPublic)
	note, _ := NewNote("33333333-3333-4333-8333-333333333333", space.ID(), "", "Agent Loop", "reviewed")
	repository := &noteRepositoryStub{found: note, trashEntry: TrashEntry{Note: note, TrashedAt: time.Now()}}
	service := NewService(ServiceConfig{
		Notes:      repository,
		Knowledge:  &knowledgeCatalogStub{space: space},
		GenerateID: idSequence("44444444-4444-4444-8444-444444444444"),
		Now:        func() time.Time { return time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC) },
	})
	if err := service.TrashNote(context.Background(), note.ID(), 1); err != nil {
		t.Fatalf("TrashNote() error = %v", err)
	}
	if repository.trashedID != note.ID() {
		t.Errorf("trashed id = %q", repository.trashedID)
	}
	if _, err := service.RestoreFromTrash(context.Background(), note.ID(), RestoreTrashInput{}); !errors.Is(err, ErrPublicRestoreConfirmationRequired) {
		t.Fatalf("RestoreFromTrash() error = %v, want %v", err, ErrPublicRestoreConfirmationRequired)
	}
	restored, err := service.RestoreFromTrash(context.Background(), note.ID(), RestoreTrashInput{ConfirmPublish: true})
	if err != nil {
		t.Fatalf("confirmed RestoreFromTrash() error = %v", err)
	}
	if restored.ID() != note.ID() || repository.trashRestoreRevision == nil || repository.trashRestoreRevision.Reason != RevisionReasonPublish {
		t.Errorf("restored = %+v, revision = %+v", restored, repository.trashRestoreRevision)
	}
	if err := service.DeleteTrashedNote(context.Background(), note.ID()); err != nil {
		t.Fatalf("DeleteTrashedNote() error = %v", err)
	}
	if repository.deletedTrashID != note.ID() {
		t.Errorf("deleted trash id = %q", repository.deletedTrashID)
	}
}

func TestServiceProjectsCurrentAndPublishedLinksFromCanonicalMarkdown(t *testing.T) {
	space, _ := knowledge.NewSpace("11111111-1111-4111-8111-111111111111", "AI Agent", knowledge.VisibilityPublic)
	repository := &noteRepositoryStub{}
	projector := &linkProjectorStub{}
	searchProjector := &searchProjectorStub{}
	attachmentProjector := &attachmentProjectorStub{}
	service := NewService(ServiceConfig{
		Notes:       repository,
		Knowledge:   &knowledgeCatalogStub{space: space},
		GenerateID:  idSequence("33333333-3333-4333-8333-333333333333", "44444444-4444-4444-8444-444444444444"),
		Now:         time.Now,
		Links:       projector,
		Search:      searchProjector,
		Attachments: attachmentProjector,
	})
	note, err := service.CreateNote(context.Background(), space.ID(), "", "Agent Loop", "[Memory](note:22222222-2222-4222-8222-222222222222)\n\n![diagram](attachment:55555555-5555-4555-8555-555555555555)")
	if err != nil {
		t.Fatalf("CreateNote() error = %v", err)
	}
	if len(projector.currentTargets) != 1 || projector.currentTargets[0] != "22222222-2222-4222-8222-222222222222" {
		t.Errorf("current targets = %+v", projector.currentTargets)
	}
	if searchProjector.currentMarkdown != note.Markdown() {
		t.Errorf("current search Markdown = %q", searchProjector.currentMarkdown)
	}
	repository.found = note
	if _, err := service.Publish(context.Background(), note.ID(), 1); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if len(projector.publishedTargets) != 1 || projector.publishedTargets[0] != projector.currentTargets[0] {
		t.Errorf("published targets = %+v", projector.publishedTargets)
	}
	if searchProjector.publishedMarkdown != note.Published().Markdown {
		t.Errorf("published search Markdown = %q", searchProjector.publishedMarkdown)
	}
	if len(attachmentProjector.publishedIDs) != 1 || attachmentProjector.publishedIDs[0] != "55555555-5555-4555-8555-555555555555" {
		t.Errorf("published Attachment ids = %+v", attachmentProjector.publishedIDs)
	}
}

func TestServiceMovingPrivateNoteToPublicRequiresAtomicPublishConfirmation(t *testing.T) {
	privateSpace, _ := knowledge.NewSpace("11111111-1111-4111-8111-111111111111", "Private", knowledge.VisibilityPrivate)
	publicSpace, _ := knowledge.NewSpace("22222222-2222-4222-8222-222222222222", "Public", knowledge.VisibilityPublic)
	note, _ := NewNote("33333333-3333-4333-8333-333333333333", privateSpace.ID(), "", "Memory", "reviewed")
	repository := &noteRepositoryStub{found: note}
	service := NewService(ServiceConfig{
		Notes: repository,
		Knowledge: &knowledgeCatalogStub{spaces: map[string]*knowledge.Space{
			privateSpace.ID(): privateSpace,
			publicSpace.ID():  publicSpace,
		}},
		GenerateID: idSequence("44444444-4444-4444-8444-444444444444"),
		Now:        time.Now,
	})
	if _, err := service.Move(context.Background(), note.ID(), publicSpace.ID(), "", 1, false); !errors.Is(err, ErrPublicMoveConfirmationRequired) {
		t.Fatalf("Move(unconfirmed) error = %v, want %v", err, ErrPublicMoveConfirmationRequired)
	}
	moved, err := service.Move(context.Background(), note.ID(), publicSpace.ID(), "", 1, true)
	if err != nil {
		t.Fatalf("Move(confirmed) error = %v", err)
	}
	if moved.ID() != note.ID() || moved.SpaceID() != publicSpace.ID() || moved.Published() == nil || repository.moveRevision == nil {
		t.Errorf("moved = %+v, revision = %+v", moved, repository.moveRevision)
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
	notePage               NotePage
	publishedNote          PublishedNote
	publishedNotePage      PublishedNotePage
	trashedID              string
	trashEntry             TrashEntry
	trashPage              TrashPage
	trashRestoreNote       *Note
	trashRestoreRevision   *Revision
	deletedTrashID         string
	moved                  *Note
	moveRevision           *Revision
}

func (repository *noteRepositoryStub) CreateNote(_ context.Context, note *Note) error {
	repository.created = note
	return nil
}

func (repository *noteRepositoryStub) GetNote(context.Context, string) (*Note, error) {
	return repository.found, nil
}

func (repository *noteRepositoryStub) ListNotes(context.Context, NoteListFilter) (NotePage, error) {
	return repository.notePage, nil
}

func (repository *noteRepositoryStub) GetPublishedNote(context.Context, string) (PublishedNote, error) {
	return repository.publishedNote, nil
}

func (repository *noteRepositoryStub) ListPublishedNotes(context.Context, string, int, int) (PublishedNotePage, error) {
	return repository.publishedNotePage, nil
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

func (repository *noteRepositoryStub) Move(_ context.Context, note *Note, revision *Revision, _ int64) error {
	repository.moved = note
	repository.moveRevision = revision
	return nil
}

func (repository *noteRepositoryStub) TrashNote(_ context.Context, id string, _ int64, _ time.Time) error {
	repository.trashedID = id
	return nil
}

func (repository *noteRepositoryStub) ListTrash(context.Context, int, int) (TrashPage, error) {
	return repository.trashPage, nil
}

func (repository *noteRepositoryStub) GetTrashedNote(context.Context, string) (TrashEntry, error) {
	return repository.trashEntry, nil
}

func (repository *noteRepositoryStub) RestoreFromTrash(_ context.Context, note *Note, revision *Revision) error {
	repository.trashRestoreNote = note
	repository.trashRestoreRevision = revision
	return nil
}

func (repository *noteRepositoryStub) DeleteTrashedNote(_ context.Context, id string) error {
	repository.deletedTrashID = id
	return nil
}

type knowledgeCatalogStub struct {
	space     *knowledge.Space
	spaces    map[string]*knowledge.Space
	directory *knowledge.Directory
}

type linkProjectorStub struct {
	currentTargets   []string
	publishedTargets []string
}

type searchProjectorStub struct {
	currentMarkdown   string
	publishedMarkdown string
}

type attachmentProjectorStub struct {
	publishedIDs []string
}

func (projector *attachmentProjectorStub) ReplacePublishedNoteAttachments(_ context.Context, _ string, ids []string, _ time.Time) error {
	projector.publishedIDs = ids
	return nil
}

func (projector *searchProjectorStub) ProjectCurrentNote(_ context.Context, _, _, markdown string) error {
	projector.currentMarkdown = markdown
	return nil
}

func (projector *searchProjectorStub) ProjectPublishedNote(_ context.Context, _, _, markdown string) error {
	projector.publishedMarkdown = markdown
	return nil
}

func (projector *linkProjectorStub) ReplaceCurrentNoteLinks(_ context.Context, _ string, targets []string) error {
	projector.currentTargets = targets
	return nil
}

func (projector *linkProjectorStub) ReplacePublishedNoteLinks(_ context.Context, _ string, targets []string) error {
	projector.publishedTargets = targets
	return nil
}

func (catalog *knowledgeCatalogStub) GetSpace(_ context.Context, id string) (*knowledge.Space, error) {
	if catalog.spaces != nil {
		return catalog.spaces[id], nil
	}
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
