package knowledge

import (
	"context"
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

func (repository *spaceRepositoryStub) GetSpace(context.Context, string) (*Space, error) {
	return repository.found, nil
}

func (repository *spaceRepositoryStub) UpdateSpace(_ context.Context, space *Space) error {
	repository.updated = space
	return nil
}
