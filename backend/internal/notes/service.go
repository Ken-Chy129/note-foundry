package notes

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Ken-Chy129/note-foundry/backend/internal/knowledge"
)

var (
	ErrInvalidNoteDirectory = errors.New("Learning Note directory belongs to another Knowledge Space")
	ErrPrivateNotePublish   = errors.New("private Learning Notes do not have Published Content")
)

type Repository interface {
	CreateNote(context.Context, *Note) error
	GetNote(context.Context, string) (*Note, error)
	ListNotes(context.Context, NoteListFilter) (NotePage, error)
	GetPublishedNote(context.Context, string) (PublishedNote, error)
	ListPublishedNotes(context.Context, string, int, int) (PublishedNotePage, error)
	UpdateDraft(context.Context, *Note, int64) error
	Publish(context.Context, *Note, Revision) error
	CreateCheckpoint(context.Context, string, int64, Revision) error
	ListRevisions(context.Context, string, int, int) (RevisionPage, error)
	GetRevision(context.Context, string, string) (Revision, error)
	Restore(context.Context, *Note, Revision, int64) error
	TrashNote(context.Context, string, int64, time.Time) error
	ListTrash(context.Context, int, int) (TrashPage, error)
	GetTrashedNote(context.Context, string) (TrashEntry, error)
	RestoreFromTrash(context.Context, *Note, *Revision) error
	DeleteTrashedNote(context.Context, string) error
}

type KnowledgeCatalog interface {
	GetSpace(context.Context, string) (*knowledge.Space, error)
	GetDirectory(context.Context, string) (*knowledge.Directory, error)
}

type LinkProjector interface {
	ReplaceCurrentNoteLinks(context.Context, string, []string) error
	ReplacePublishedNoteLinks(context.Context, string, []string) error
}

type SearchProjector interface {
	ProjectCurrentNote(context.Context, string, string, string) error
	ProjectPublishedNote(context.Context, string, string, string) error
}

type IDGenerator func() string

type ServiceConfig struct {
	Notes      Repository
	Knowledge  KnowledgeCatalog
	GenerateID IDGenerator
	Now        func() time.Time
	Links      LinkProjector
	Search     SearchProjector
}

type Service struct {
	notes      Repository
	knowledge  KnowledgeCatalog
	generateID IDGenerator
	now        func() time.Time
	links      LinkProjector
	search     SearchProjector
}

type RestoreTrashInput struct {
	SpaceID           string
	DirectoryID       string
	LocationSpecified bool
	ConfirmPublish    bool
}

func NewService(config ServiceConfig) *Service {
	return &Service{
		notes:      config.Notes,
		knowledge:  config.Knowledge,
		generateID: config.GenerateID,
		now:        config.Now,
		links:      config.Links,
		search:     config.Search,
	}
}

func (service *Service) CreateNote(ctx context.Context, spaceID, directoryID, title, markdown string) (*Note, error) {
	if _, err := service.knowledge.GetSpace(ctx, spaceID); err != nil {
		return nil, fmt.Errorf("load Learning Note Knowledge Space: %w", err)
	}
	if directoryID != "" {
		directory, err := service.knowledge.GetDirectory(ctx, directoryID)
		if err != nil {
			return nil, fmt.Errorf("load Learning Note directory: %w", err)
		}
		if directory.SpaceID() != spaceID {
			return nil, ErrInvalidNoteDirectory
		}
	}

	note, err := NewNote(service.generateID(), spaceID, directoryID, title, markdown)
	if err != nil {
		return nil, err
	}
	if err := service.notes.CreateNote(ctx, note); err != nil {
		return nil, fmt.Errorf("create Learning Note: %w", err)
	}
	if err := service.replaceCurrentLinks(ctx, note); err != nil {
		return nil, err
	}
	if err := service.projectCurrentSearch(ctx, note); err != nil {
		return nil, err
	}
	return note, nil
}

func (service *Service) GetNote(ctx context.Context, id string) (*Note, error) {
	note, err := service.notes.GetNote(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("load Learning Note: %w", err)
	}
	return note, nil
}

func (service *Service) ListNotes(ctx context.Context, filter NoteListFilter) (NotePage, error) {
	page, err := service.notes.ListNotes(ctx, filter)
	if err != nil {
		return NotePage{}, fmt.Errorf("list Learning Notes: %w", err)
	}
	return page, nil
}

func (service *Service) GetPublishedNote(ctx context.Context, id string) (PublishedNote, error) {
	note, err := service.notes.GetPublishedNote(ctx, id)
	if err != nil {
		return PublishedNote{}, fmt.Errorf("load published Learning Note: %w", err)
	}
	return note, nil
}

func (service *Service) ListPublishedNotes(ctx context.Context, spaceID string, page, pageSize int) (PublishedNotePage, error) {
	notes, err := service.notes.ListPublishedNotes(ctx, spaceID, page, pageSize)
	if err != nil {
		return PublishedNotePage{}, fmt.Errorf("list published Learning Notes: %w", err)
	}
	return notes, nil
}

func (service *Service) Autosave(ctx context.Context, id string, expectedVersion int64, title, markdown string) (*Note, error) {
	note, err := service.notes.GetNote(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("load Learning Note: %w", err)
	}
	if err := note.Autosave(expectedVersion, title, markdown); err != nil {
		return nil, err
	}
	if err := service.notes.UpdateDraft(ctx, note, expectedVersion); err != nil {
		return nil, fmt.Errorf("save Learning Note draft: %w", err)
	}
	if err := service.replaceCurrentLinks(ctx, note); err != nil {
		return nil, err
	}
	if err := service.projectCurrentSearch(ctx, note); err != nil {
		return nil, err
	}
	return note, nil
}

func (service *Service) Publish(ctx context.Context, id string, expectedVersion int64) (*Note, error) {
	note, err := service.notes.GetNote(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("load Learning Note: %w", err)
	}
	if note.Version() != expectedVersion {
		return nil, ErrVersionConflict
	}
	space, err := service.knowledge.GetSpace(ctx, note.SpaceID())
	if err != nil {
		return nil, fmt.Errorf("load Learning Note Knowledge Space: %w", err)
	}
	if space.Visibility() != knowledge.VisibilityPublic {
		return nil, ErrPrivateNotePublish
	}

	revision, err := note.Publish(service.generateID(), service.now())
	if err != nil {
		return nil, err
	}
	if err := service.notes.Publish(ctx, note, revision); err != nil {
		return nil, fmt.Errorf("publish Learning Note: %w", err)
	}
	if err := service.replacePublishedLinks(ctx, note); err != nil {
		return nil, err
	}
	if err := service.projectPublishedSearch(ctx, note); err != nil {
		return nil, err
	}
	return note, nil
}

func (service *Service) CreateCheckpoint(ctx context.Context, id string, expectedVersion int64) (Revision, error) {
	note, err := service.notes.GetNote(ctx, id)
	if err != nil {
		return Revision{}, fmt.Errorf("load Learning Note: %w", err)
	}
	revision, err := note.Checkpoint(expectedVersion, service.generateID(), RevisionReasonManual, service.now())
	if err != nil {
		return Revision{}, err
	}
	if err := service.notes.CreateCheckpoint(ctx, note.ID(), expectedVersion, revision); err != nil {
		return Revision{}, fmt.Errorf("create Note Revision checkpoint: %w", err)
	}
	return revision, nil
}

func (service *Service) ListRevisions(ctx context.Context, id string, page, pageSize int) (RevisionPage, error) {
	revisions, err := service.notes.ListRevisions(ctx, id, page, pageSize)
	if err != nil {
		return RevisionPage{}, fmt.Errorf("list Note Revisions: %w", err)
	}
	return revisions, nil
}

func (service *Service) Restore(ctx context.Context, id, revisionID string, expectedVersion int64) (*Note, error) {
	note, err := service.notes.GetNote(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("load Learning Note: %w", err)
	}
	target, err := service.notes.GetRevision(ctx, id, revisionID)
	if err != nil {
		return nil, fmt.Errorf("load Note Revision: %w", err)
	}
	checkpoint, err := note.Restore(expectedVersion, target, service.generateID(), service.now())
	if err != nil {
		return nil, err
	}
	if err := service.notes.Restore(ctx, note, checkpoint, expectedVersion); err != nil {
		return nil, fmt.Errorf("restore Note Revision: %w", err)
	}
	if err := service.replaceCurrentLinks(ctx, note); err != nil {
		return nil, err
	}
	if err := service.projectCurrentSearch(ctx, note); err != nil {
		return nil, err
	}
	return note, nil
}

func (service *Service) TrashNote(ctx context.Context, id string, expectedVersion int64) error {
	if _, err := service.notes.GetNote(ctx, id); err != nil {
		return fmt.Errorf("load Learning Note: %w", err)
	}
	if err := service.notes.TrashNote(ctx, id, expectedVersion, service.now()); err != nil {
		return fmt.Errorf("move Learning Note to Trash: %w", err)
	}
	return nil
}

func (service *Service) ListTrash(ctx context.Context, page, pageSize int) (TrashPage, error) {
	entries, err := service.notes.ListTrash(ctx, page, pageSize)
	if err != nil {
		return TrashPage{}, fmt.Errorf("list Trash: %w", err)
	}
	return entries, nil
}

func (service *Service) RestoreFromTrash(ctx context.Context, id string, input RestoreTrashInput) (*Note, error) {
	entry, err := service.notes.GetTrashedNote(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("load trashed Learning Note: %w", err)
	}
	spaceID := input.SpaceID
	if spaceID == "" {
		spaceID = entry.Note.SpaceID()
	}
	space, err := service.knowledge.GetSpace(ctx, spaceID)
	if err != nil {
		return nil, fmt.Errorf("load restore Knowledge Space: %w", err)
	}
	directoryID := input.DirectoryID
	if !input.LocationSpecified {
		directoryID = entry.Note.DirectoryID()
	}
	if directoryID != "" {
		directory, err := service.knowledge.GetDirectory(ctx, directoryID)
		if err != nil {
			return nil, fmt.Errorf("load restore directory: %w", err)
		}
		if directory.SpaceID() != spaceID {
			return nil, ErrInvalidNoteDirectory
		}
	}
	if err := entry.Note.Relocate(spaceID, directoryID); err != nil {
		return nil, err
	}
	var revision *Revision
	if space.Visibility() == knowledge.VisibilityPublic {
		if !input.ConfirmPublish {
			return nil, ErrPublicRestoreConfirmationRequired
		}
		publishedRevision, err := entry.Note.Publish(service.generateID(), service.now())
		if err != nil {
			return nil, err
		}
		revision = &publishedRevision
	} else {
		entry.Note.MakePrivate()
	}
	if err := service.notes.RestoreFromTrash(ctx, entry.Note, revision); err != nil {
		return nil, fmt.Errorf("restore Learning Note from Trash: %w", err)
	}
	if err := service.replaceCurrentLinks(ctx, entry.Note); err != nil {
		return nil, err
	}
	if revision != nil {
		if err := service.replacePublishedLinks(ctx, entry.Note); err != nil {
			return nil, err
		}
	}
	if err := service.projectCurrentSearch(ctx, entry.Note); err != nil {
		return nil, err
	}
	if revision != nil {
		if err := service.projectPublishedSearch(ctx, entry.Note); err != nil {
			return nil, err
		}
	}
	return entry.Note, nil
}

func (service *Service) DeleteTrashedNote(ctx context.Context, id string) error {
	if err := service.notes.DeleteTrashedNote(ctx, id); err != nil {
		return fmt.Errorf("permanently delete Learning Note: %w", err)
	}
	return nil
}

func (service *Service) replaceCurrentLinks(ctx context.Context, note *Note) error {
	if service.links == nil {
		return nil
	}
	if err := service.links.ReplaceCurrentNoteLinks(ctx, note.ID(), ExtractNoteLinkTargets(note.Markdown())); err != nil {
		return fmt.Errorf("project current Note Links: %w", err)
	}
	return nil
}

func (service *Service) replacePublishedLinks(ctx context.Context, note *Note) error {
	if service.links == nil || note.Published() == nil {
		return nil
	}
	if err := service.links.ReplacePublishedNoteLinks(ctx, note.ID(), ExtractNoteLinkTargets(note.Published().Markdown)); err != nil {
		return fmt.Errorf("project published Note Links: %w", err)
	}
	return nil
}

func (service *Service) projectCurrentSearch(ctx context.Context, note *Note) error {
	if service.search == nil {
		return nil
	}
	if err := service.search.ProjectCurrentNote(ctx, note.ID(), note.Title(), note.Markdown()); err != nil {
		return fmt.Errorf("project current Learning Note search: %w", err)
	}
	return nil
}

func (service *Service) projectPublishedSearch(ctx context.Context, note *Note) error {
	if service.search == nil || note.Published() == nil {
		return nil
	}
	if err := service.search.ProjectPublishedNote(ctx, note.ID(), note.Published().Title, note.Published().Markdown); err != nil {
		return fmt.Errorf("project published Learning Note search: %w", err)
	}
	return nil
}
