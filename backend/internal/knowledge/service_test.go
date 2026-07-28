package knowledge

import (
	"context"
	"errors"
	"testing"
)

func TestServiceCreatesKnowledgeSpaceWithGeneratedStableIdentity(t *testing.T) {
	repository := &spaceRepositoryStub{}
	service := NewService(ServiceConfig{
		Spaces:     repository,
		GenerateID: func() string { return "11111111-1111-4111-8111-111111111111" },
	})

	space, err := service.CreateSpace(context.Background(), "  AI Agent  ", VisibilityPublic)
	if err != nil {
		t.Fatalf("CreateSpace() error = %v", err)
	}
	if space.ID() != "11111111-1111-4111-8111-111111111111" {
		t.Errorf("space ID = %q", space.ID())
	}
	if space.Name() != "AI Agent" || space.Visibility() != VisibilityPublic {
		t.Errorf("space = name:%q visibility:%q", space.Name(), space.Visibility())
	}
	if repository.created != space {
		t.Error("created space was not persisted")
	}
}

func TestServiceCreatesNestedDirectoryOnlyInsideItsKnowledgeSpace(t *testing.T) {
	space, _ := NewSpace("11111111-1111-4111-8111-111111111111", "AI Agent", VisibilityPublic)
	parent, _ := NewDirectory("22222222-2222-4222-8222-222222222222", space.ID(), "", "Agents")
	directories := &directoryRepositoryStub{found: parent}
	service := NewService(ServiceConfig{
		Spaces:      &spaceRepositoryStub{found: space},
		Directories: directories,
		GenerateID:  func() string { return "33333333-3333-4333-8333-333333333333" },
	})

	directory, err := service.CreateDirectory(context.Background(), space.ID(), parent.ID(), "  Hermes Agent  ")
	if err != nil {
		t.Fatalf("CreateDirectory() error = %v", err)
	}
	if directory.SpaceID() != space.ID() || directory.ParentID() != parent.ID() || directory.Name() != "Hermes Agent" {
		t.Errorf("directory = space:%q parent:%q name:%q", directory.SpaceID(), directory.ParentID(), directory.Name())
	}
	if directories.created != directory {
		t.Error("directory was not persisted")
	}
}

func TestServiceRejectsDirectoryParentFromAnotherKnowledgeSpace(t *testing.T) {
	space, _ := NewSpace("11111111-1111-4111-8111-111111111111", "AI Agent", VisibilityPublic)
	parent, _ := NewDirectory("22222222-2222-4222-8222-222222222222", "99999999-9999-4999-8999-999999999999", "", "Other")
	service := NewService(ServiceConfig{
		Spaces:      &spaceRepositoryStub{found: space},
		Directories: &directoryRepositoryStub{found: parent},
		GenerateID:  func() string { return "33333333-3333-4333-8333-333333333333" },
	})

	_, err := service.CreateDirectory(context.Background(), space.ID(), parent.ID(), "Hermes Agent")
	if !errors.Is(err, ErrDirectoryWrongSpace) {
		t.Fatalf("CreateDirectory() error = %v, want %v", err, ErrDirectoryWrongSpace)
	}
}

func TestServiceRejectsDirectoryMoveBelowItsDescendant(t *testing.T) {
	space, _ := NewSpace("11111111-1111-4111-8111-111111111111", "AI Agent", VisibilityPublic)
	directory, _ := NewDirectory("22222222-2222-4222-8222-222222222222", space.ID(), "", "Hermes Agent")
	descendant, _ := NewDirectory("33333333-3333-4333-8333-333333333333", space.ID(), directory.ID(), "Architecture")
	directories := &directoryRepositoryStub{
		foundByID:  map[string]*Directory{directory.ID(): directory, descendant.ID(): descendant},
		wouldCycle: true,
	}
	service := NewService(ServiceConfig{
		Spaces:      &spaceRepositoryStub{found: space},
		Directories: directories,
		GenerateID:  func() string { return "unused" },
	})

	_, err := service.MoveDirectory(context.Background(), directory.ID(), descendant.ID())
	if !errors.Is(err, ErrDirectoryCycle) {
		t.Fatalf("MoveDirectory() error = %v, want %v", err, ErrDirectoryCycle)
	}
	if directories.updated != nil {
		t.Error("cyclic directory move was persisted")
	}
}

func TestServiceCreatesAndRenamesGlobalTag(t *testing.T) {
	tags := &tagRepositoryStub{}
	service := NewService(ServiceConfig{
		Spaces:      &spaceRepositoryStub{},
		Directories: &directoryRepositoryStub{},
		Tags:        tags,
		GenerateID:  func() string { return "11111111-1111-4111-8111-111111111111" },
	})

	tag, err := service.CreateTag(context.Background(), "  memory  ")
	if err != nil {
		t.Fatalf("CreateTag() error = %v", err)
	}
	if tags.created != tag || tag.Name() != "memory" {
		t.Errorf("created tag = %+v", tag)
	}
	tags.found = tag
	renamed, err := service.RenameTag(context.Background(), tag.ID(), "agent-memory")
	if err != nil {
		t.Fatalf("RenameTag() error = %v", err)
	}
	if renamed.ID() != tag.ID() || renamed.Name() != "agent-memory" || tags.updated != tag {
		t.Errorf("renamed tag = %+v", renamed)
	}
}

func TestServiceSetsDeduplicatedNoteTagsAndMergesTags(t *testing.T) {
	tag, _ := NewTag("11111111-1111-4111-8111-111111111111", "memory")
	tags := &tagRepositoryStub{tags: []*Tag{tag}, merged: tag}
	service := NewService(ServiceConfig{Tags: tags, GenerateID: func() string { return "unused" }})

	result, err := service.SetNoteTags(context.Background(), "22222222-2222-4222-8222-222222222222", []string{tag.ID(), tag.ID()})
	if err != nil {
		t.Fatalf("SetNoteTags() error = %v", err)
	}
	if len(tags.setTagIDs) != 1 || len(result) != 1 {
		t.Errorf("set ids = %+v, result = %+v", tags.setTagIDs, result)
	}
	merged, err := service.MergeTag(context.Background(), "33333333-3333-4333-8333-333333333333", tag.ID())
	if err != nil {
		t.Fatalf("MergeTag() error = %v", err)
	}
	if merged.ID() != tag.ID() {
		t.Errorf("merged Tag = %+v", merged)
	}
}

func TestServiceExposesDerivedCurrentAndPublishedNoteLinks(t *testing.T) {
	link := LinkedNote{ID: "11111111-1111-4111-8111-111111111111", SpaceID: "22222222-2222-4222-8222-222222222222", Title: "Memory", Slug: "memory"}
	links := &noteLinkRepositoryStub{links: []LinkedNote{link}}
	service := NewService(ServiceConfig{Links: links})
	if err := service.ReplaceCurrentNoteLinks(context.Background(), "source", []string{link.ID}); err != nil {
		t.Fatalf("ReplaceCurrentNoteLinks() error = %v", err)
	}
	if err := service.ReplacePublishedNoteLinks(context.Background(), "source", []string{link.ID}); err != nil {
		t.Fatalf("ReplacePublishedNoteLinks() error = %v", err)
	}
	current, err := service.ListCurrentBacklinks(context.Background(), link.ID)
	if err != nil || len(current) != 1 {
		t.Fatalf("ListCurrentBacklinks() = %+v, %v", current, err)
	}
	published, err := service.ListPublishedForwardLinks(context.Background(), "source")
	if err != nil || len(published) != 1 {
		t.Fatalf("ListPublishedForwardLinks() = %+v, %v", published, err)
	}
}

func TestServiceRenamesSpaceWithoutChangingIdentity(t *testing.T) {
	space, _ := NewSpace("11111111-1111-4111-8111-111111111111", "AI", VisibilityPrivate)
	repository := &spaceRepositoryStub{found: space}
	service := NewService(ServiceConfig{
		Spaces:     repository,
		GenerateID: func() string { return "unused" },
	})

	updated, err := service.RenameSpace(context.Background(), space.ID(), "  AI Agent  ")
	if err != nil {
		t.Fatalf("RenameSpace() error = %v", err)
	}
	if updated.ID() != "11111111-1111-4111-8111-111111111111" || updated.Name() != "AI Agent" {
		t.Errorf("updated space = id:%q name:%q", updated.ID(), updated.Name())
	}
	if repository.updated != space {
		t.Error("renamed space was not persisted")
	}
}

type spaceRepositoryStub struct {
	created *Space
	found   *Space
	updated *Space
	spaces  []*Space
}

func (repository *spaceRepositoryStub) CreateSpace(_ context.Context, space *Space) error {
	repository.created = space
	return nil
}

func (repository *spaceRepositoryStub) ListSpaces(context.Context, int, int) ([]*Space, int, error) {
	return repository.spaces, len(repository.spaces), nil
}

func (repository *spaceRepositoryStub) ListPublicSpaces(context.Context, int, int) ([]*Space, int, error) {
	return repository.spaces, len(repository.spaces), nil
}

func (repository *spaceRepositoryStub) GetSpace(context.Context, string) (*Space, error) {
	return repository.found, nil
}

func (repository *spaceRepositoryStub) GetPublicSpace(context.Context, string) (*Space, error) {
	return repository.found, nil
}

func (repository *spaceRepositoryStub) UpdateSpace(_ context.Context, space *Space) error {
	repository.updated = space
	return nil
}

type directoryRepositoryStub struct {
	created     *Directory
	found       *Directory
	foundByID   map[string]*Directory
	updated     *Directory
	directories []*Directory
	wouldCycle  bool
}

func (repository *directoryRepositoryStub) CreateDirectory(_ context.Context, directory *Directory) error {
	repository.created = directory
	return nil
}

func (repository *directoryRepositoryStub) ListDirectories(context.Context, string) ([]*Directory, error) {
	return repository.directories, nil
}

func (repository *directoryRepositoryStub) ListPublicDirectories(context.Context, string) ([]*Directory, error) {
	return repository.directories, nil
}

func (repository *directoryRepositoryStub) GetDirectory(_ context.Context, id string) (*Directory, error) {
	if repository.foundByID != nil {
		return repository.foundByID[id], nil
	}
	return repository.found, nil
}

func (repository *directoryRepositoryStub) UpdateDirectory(_ context.Context, directory *Directory) error {
	repository.updated = directory
	return nil
}

func (repository *directoryRepositoryStub) WouldCreateDirectoryCycle(context.Context, string, string) (bool, error) {
	return repository.wouldCycle, nil
}

type tagRepositoryStub struct {
	created   *Tag
	found     *Tag
	updated   *Tag
	tags      []*Tag
	setTagIDs []string
	merged    *Tag
}

func (repository *tagRepositoryStub) CreateTag(_ context.Context, tag *Tag) error {
	repository.created = tag
	return nil
}

func (repository *tagRepositoryStub) ListTags(context.Context, int, int) ([]*Tag, int, error) {
	return repository.tags, len(repository.tags), nil
}

func (repository *tagRepositoryStub) GetTag(context.Context, string) (*Tag, error) {
	return repository.found, nil
}

func (repository *tagRepositoryStub) UpdateTag(_ context.Context, tag *Tag) error {
	repository.updated = tag
	return nil
}

func (repository *tagRepositoryStub) SetNoteTags(_ context.Context, _ string, tagIDs []string) error {
	repository.setTagIDs = tagIDs
	return nil
}

func (repository *tagRepositoryStub) ListNoteTags(context.Context, string) ([]*Tag, error) {
	return repository.tags, nil
}

func (repository *tagRepositoryStub) MergeTag(context.Context, string, string) (*Tag, error) {
	return repository.merged, nil
}

type noteLinkRepositoryStub struct {
	links []LinkedNote
}

func (repository *noteLinkRepositoryStub) ReplaceCurrentNoteLinks(context.Context, string, []string) error {
	return nil
}

func (repository *noteLinkRepositoryStub) ReplacePublishedNoteLinks(context.Context, string, []string) error {
	return nil
}

func (repository *noteLinkRepositoryStub) ListCurrentForwardLinks(context.Context, string) ([]LinkedNote, error) {
	return repository.links, nil
}

func (repository *noteLinkRepositoryStub) ListCurrentBacklinks(context.Context, string) ([]LinkedNote, error) {
	return repository.links, nil
}

func (repository *noteLinkRepositoryStub) ListPublishedForwardLinks(context.Context, string) ([]LinkedNote, error) {
	return repository.links, nil
}

func (repository *noteLinkRepositoryStub) ListPublishedBacklinks(context.Context, string) ([]LinkedNote, error) {
	return repository.links, nil
}
