package knowledge

import (
	"context"
	"errors"
	"fmt"
)

var (
	ErrDirectoryWrongSpace = errors.New("directory parent belongs to another Knowledge Space")
	ErrDirectoryCycle      = errors.New("directory move would create a cycle")
)

type SpaceRepository interface {
	CreateSpace(context.Context, *Space) error
	ListSpaces(context.Context, int, int) ([]*Space, int, error)
	ListPublicSpaces(context.Context, int, int) ([]*Space, int, error)
	GetSpace(context.Context, string) (*Space, error)
	GetPublicSpace(context.Context, string) (*Space, error)
	UpdateSpace(context.Context, *Space) error
}

type DirectoryRepository interface {
	CreateDirectory(context.Context, *Directory) error
	ListDirectories(context.Context, string) ([]*Directory, error)
	ListPublicDirectories(context.Context, string) ([]*Directory, error)
	GetDirectory(context.Context, string) (*Directory, error)
	UpdateDirectory(context.Context, *Directory) error
	WouldCreateDirectoryCycle(context.Context, string, string) (bool, error)
}

type TagRepository interface {
	CreateTag(context.Context, *Tag) error
	ListTags(context.Context, int, int) ([]*Tag, int, error)
	GetTag(context.Context, string) (*Tag, error)
	UpdateTag(context.Context, *Tag) error
	SetNoteTags(context.Context, string, []string) error
	ListNoteTags(context.Context, string) ([]*Tag, error)
	MergeTag(context.Context, string, string) (*Tag, error)
}

type NoteLinkRepository interface {
	ReplaceCurrentNoteLinks(context.Context, string, []string) error
	ReplacePublishedNoteLinks(context.Context, string, []string) error
	ListCurrentForwardLinks(context.Context, string) ([]LinkedNote, error)
	ListCurrentBacklinks(context.Context, string) ([]LinkedNote, error)
	ListPublishedForwardLinks(context.Context, string) ([]LinkedNote, error)
	ListPublishedBacklinks(context.Context, string) ([]LinkedNote, error)
}

type TagSearchProjector interface {
	RefreshNoteTags(context.Context, string) error
	RefreshNotesForTag(context.Context, string) error
}

type IDGenerator func() string

type ServiceConfig struct {
	Spaces      SpaceRepository
	Directories DirectoryRepository
	Tags        TagRepository
	GenerateID  IDGenerator
	Links       NoteLinkRepository
	Search      TagSearchProjector
}

type Service struct {
	spaces      SpaceRepository
	directories DirectoryRepository
	tags        TagRepository
	generateID  IDGenerator
	links       NoteLinkRepository
	search      TagSearchProjector
}

type SpacePage struct {
	Spaces     []*Space
	Page       int
	PageSize   int
	TotalItems int
}

type TagPage struct {
	Tags       []*Tag
	Page       int
	PageSize   int
	TotalItems int
}

func NewService(config ServiceConfig) *Service {
	return &Service{spaces: config.Spaces, directories: config.Directories, tags: config.Tags, generateID: config.GenerateID, links: config.Links, search: config.Search}
}

func (service *Service) ReplaceCurrentNoteLinks(ctx context.Context, sourceID string, targetIDs []string) error {
	if err := service.links.ReplaceCurrentNoteLinks(ctx, sourceID, targetIDs); err != nil {
		return fmt.Errorf("replace current Note Links: %w", err)
	}
	return nil
}

func (service *Service) ReplacePublishedNoteLinks(ctx context.Context, sourceID string, targetIDs []string) error {
	if err := service.links.ReplacePublishedNoteLinks(ctx, sourceID, targetIDs); err != nil {
		return fmt.Errorf("replace published Note Links: %w", err)
	}
	return nil
}

func (service *Service) ListCurrentForwardLinks(ctx context.Context, noteID string) ([]LinkedNote, error) {
	return service.listLinks(ctx, noteID, service.links.ListCurrentForwardLinks, "current forward")
}

func (service *Service) ListCurrentBacklinks(ctx context.Context, noteID string) ([]LinkedNote, error) {
	return service.listLinks(ctx, noteID, service.links.ListCurrentBacklinks, "current back")
}

func (service *Service) ListPublishedForwardLinks(ctx context.Context, noteID string) ([]LinkedNote, error) {
	return service.listLinks(ctx, noteID, service.links.ListPublishedForwardLinks, "published forward")
}

func (service *Service) ListPublishedBacklinks(ctx context.Context, noteID string) ([]LinkedNote, error) {
	return service.listLinks(ctx, noteID, service.links.ListPublishedBacklinks, "published back")
}

func (service *Service) listLinks(ctx context.Context, noteID string, query func(context.Context, string) ([]LinkedNote, error), kind string) ([]LinkedNote, error) {
	links, err := query(ctx, noteID)
	if err != nil {
		return nil, fmt.Errorf("list %s Note Links: %w", kind, err)
	}
	return links, nil
}

func (service *Service) CreateTag(ctx context.Context, name string) (*Tag, error) {
	tag, err := NewTag(service.generateID(), name)
	if err != nil {
		return nil, err
	}
	if err := service.tags.CreateTag(ctx, tag); err != nil {
		return nil, fmt.Errorf("create tag: %w", err)
	}
	return tag, nil
}

func (service *Service) ListTags(ctx context.Context, page, pageSize int) (TagPage, error) {
	tags, total, err := service.tags.ListTags(ctx, pageSize, (page-1)*pageSize)
	if err != nil {
		return TagPage{}, fmt.Errorf("list tags: %w", err)
	}
	return TagPage{Tags: tags, Page: page, PageSize: pageSize, TotalItems: total}, nil
}

func (service *Service) RenameTag(ctx context.Context, id, name string) (*Tag, error) {
	tag, err := service.tags.GetTag(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("load tag: %w", err)
	}
	if err := tag.Rename(name); err != nil {
		return nil, err
	}
	if err := service.tags.UpdateTag(ctx, tag); err != nil {
		return nil, fmt.Errorf("update tag: %w", err)
	}
	if service.search != nil {
		if err := service.search.RefreshNotesForTag(ctx, tag.ID()); err != nil {
			return nil, fmt.Errorf("refresh search after Tag rename: %w", err)
		}
	}
	return tag, nil
}

func (service *Service) SetNoteTags(ctx context.Context, noteID string, tagIDs []string) ([]*Tag, error) {
	uniqueIDs := make([]string, 0, len(tagIDs))
	seen := make(map[string]struct{}, len(tagIDs))
	for _, tagID := range tagIDs {
		if _, exists := seen[tagID]; exists {
			continue
		}
		seen[tagID] = struct{}{}
		uniqueIDs = append(uniqueIDs, tagID)
	}
	if err := service.tags.SetNoteTags(ctx, noteID, uniqueIDs); err != nil {
		return nil, fmt.Errorf("set Learning Note Tags: %w", err)
	}
	if service.search != nil {
		if err := service.search.RefreshNoteTags(ctx, noteID); err != nil {
			return nil, fmt.Errorf("refresh search after setting Learning Note Tags: %w", err)
		}
	}
	tags, err := service.tags.ListNoteTags(ctx, noteID)
	if err != nil {
		return nil, fmt.Errorf("load Learning Note Tags after update: %w", err)
	}
	return tags, nil
}

func (service *Service) ListNoteTags(ctx context.Context, noteID string) ([]*Tag, error) {
	tags, err := service.tags.ListNoteTags(ctx, noteID)
	if err != nil {
		return nil, fmt.Errorf("list Learning Note Tags: %w", err)
	}
	return tags, nil
}

func (service *Service) MergeTag(ctx context.Context, sourceID, targetID string) (*Tag, error) {
	tag, err := service.tags.MergeTag(ctx, sourceID, targetID)
	if err != nil {
		return nil, fmt.Errorf("merge Tag: %w", err)
	}
	if service.search != nil {
		if err := service.search.RefreshNotesForTag(ctx, tag.ID()); err != nil {
			return nil, fmt.Errorf("refresh search after Tag merge: %w", err)
		}
	}
	return tag, nil
}

func (service *Service) CreateDirectory(ctx context.Context, spaceID, parentID, name string) (*Directory, error) {
	if _, err := service.spaces.GetSpace(ctx, spaceID); err != nil {
		return nil, fmt.Errorf("load directory Knowledge Space: %w", err)
	}
	if parentID != "" {
		parent, err := service.directories.GetDirectory(ctx, parentID)
		if err != nil {
			return nil, fmt.Errorf("load parent directory: %w", err)
		}
		if parent.SpaceID() != spaceID {
			return nil, ErrDirectoryWrongSpace
		}
	}

	directory, err := NewDirectory(service.generateID(), spaceID, parentID, name)
	if err != nil {
		return nil, err
	}
	if err := service.directories.CreateDirectory(ctx, directory); err != nil {
		return nil, fmt.Errorf("create directory: %w", err)
	}
	return directory, nil
}

func (service *Service) ListDirectories(ctx context.Context, spaceID string) ([]*Directory, error) {
	if _, err := service.spaces.GetSpace(ctx, spaceID); err != nil {
		return nil, fmt.Errorf("load directory Knowledge Space: %w", err)
	}
	directories, err := service.directories.ListDirectories(ctx, spaceID)
	if err != nil {
		return nil, fmt.Errorf("list directories: %w", err)
	}
	return directories, nil
}

func (service *Service) ListPublicDirectories(ctx context.Context, spaceID string) ([]*Directory, error) {
	if _, err := service.spaces.GetPublicSpace(ctx, spaceID); err != nil {
		return nil, fmt.Errorf("load public Knowledge Space: %w", err)
	}
	directories, err := service.directories.ListPublicDirectories(ctx, spaceID)
	if err != nil {
		return nil, fmt.Errorf("list public directories: %w", err)
	}
	return directories, nil
}

func (service *Service) RenameDirectory(ctx context.Context, id, name string) (*Directory, error) {
	directory, err := service.directories.GetDirectory(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("load directory: %w", err)
	}
	if err := directory.Rename(name); err != nil {
		return nil, err
	}
	if err := service.directories.UpdateDirectory(ctx, directory); err != nil {
		return nil, fmt.Errorf("update directory: %w", err)
	}
	return directory, nil
}

func (service *Service) MoveDirectory(ctx context.Context, id, newParentID string) (*Directory, error) {
	directory, err := service.directories.GetDirectory(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("load directory: %w", err)
	}
	if newParentID != "" {
		parent, err := service.directories.GetDirectory(ctx, newParentID)
		if err != nil {
			return nil, fmt.Errorf("load target parent directory: %w", err)
		}
		if parent.SpaceID() != directory.SpaceID() {
			return nil, ErrDirectoryWrongSpace
		}
		wouldCycle, err := service.directories.WouldCreateDirectoryCycle(ctx, directory.ID(), newParentID)
		if err != nil {
			return nil, fmt.Errorf("check directory move: %w", err)
		}
		if wouldCycle {
			return nil, ErrDirectoryCycle
		}
	}
	if err := directory.Move(newParentID); err != nil {
		return nil, err
	}
	if err := service.directories.UpdateDirectory(ctx, directory); err != nil {
		return nil, fmt.Errorf("move directory: %w", err)
	}
	return directory, nil
}

func (service *Service) CreateSpace(ctx context.Context, name string, visibility Visibility) (*Space, error) {
	space, err := NewSpace(service.generateID(), name, visibility)
	if err != nil {
		return nil, err
	}
	if err := service.spaces.CreateSpace(ctx, space); err != nil {
		return nil, fmt.Errorf("create knowledge space: %w", err)
	}
	return space, nil
}

func (service *Service) ListSpaces(ctx context.Context, page, pageSize int) (SpacePage, error) {
	spaces, total, err := service.spaces.ListSpaces(ctx, pageSize, (page-1)*pageSize)
	if err != nil {
		return SpacePage{}, fmt.Errorf("list knowledge spaces: %w", err)
	}
	return SpacePage{Spaces: spaces, Page: page, PageSize: pageSize, TotalItems: total}, nil
}

func (service *Service) ListPublicSpaces(ctx context.Context, page, pageSize int) (SpacePage, error) {
	spaces, total, err := service.spaces.ListPublicSpaces(ctx, pageSize, (page-1)*pageSize)
	if err != nil {
		return SpacePage{}, fmt.Errorf("list public Knowledge Spaces: %w", err)
	}
	return SpacePage{Spaces: spaces, Page: page, PageSize: pageSize, TotalItems: total}, nil
}

func (service *Service) RenameSpace(ctx context.Context, id, name string) (*Space, error) {
	space, err := service.spaces.GetSpace(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("load knowledge space: %w", err)
	}
	if err := space.Rename(name); err != nil {
		return nil, err
	}
	if err := service.spaces.UpdateSpace(ctx, space); err != nil {
		return nil, fmt.Errorf("update knowledge space: %w", err)
	}
	return space, nil
}

func (service *Service) GetSpace(ctx context.Context, id string) (*Space, error) {
	space, err := service.spaces.GetSpace(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("load Knowledge Space: %w", err)
	}
	return space, nil
}

func (service *Service) GetDirectory(ctx context.Context, id string) (*Directory, error) {
	directory, err := service.directories.GetDirectory(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("load directory: %w", err)
	}
	return directory, nil
}
