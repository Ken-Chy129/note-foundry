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
	UpdateDraft(context.Context, *Note, int64) error
	Publish(context.Context, *Note, Revision) error
}

type KnowledgeCatalog interface {
	GetSpace(context.Context, string) (*knowledge.Space, error)
	GetDirectory(context.Context, string) (*knowledge.Directory, error)
}

type IDGenerator func() string

type ServiceConfig struct {
	Notes      Repository
	Knowledge  KnowledgeCatalog
	GenerateID IDGenerator
	Now        func() time.Time
}

type Service struct {
	notes      Repository
	knowledge  KnowledgeCatalog
	generateID IDGenerator
	now        func() time.Time
}

func NewService(config ServiceConfig) *Service {
	return &Service{
		notes:      config.Notes,
		knowledge:  config.Knowledge,
		generateID: config.GenerateID,
		now:        config.Now,
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
	return note, nil
}

func (service *Service) GetNote(ctx context.Context, id string) (*Note, error) {
	note, err := service.notes.GetNote(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("load Learning Note: %w", err)
	}
	return note, nil
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
	return note, nil
}
