package sources

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestServiceOrganizesSourceIntoExistingSpace(t *testing.T) {
	now := time.Date(2026, 7, 31, 9, 0, 0, 0, time.UTC)
	source, _ := NewManualSource("11111111-1111-4111-8111-111111111111", "Source", "", "", now.Add(-time.Hour))
	repository := &sourceRepositoryStub{source: source}
	checkedSpaceID := ""
	service := NewService(ServiceConfig{
		Repository: repository,
		Now:        func() time.Time { return now },
		SpaceExists: func(_ context.Context, spaceID string) (bool, error) {
			checkedSpaceID = spaceID
			return true, nil
		},
	})

	organized, err := service.OrganizeSource(context.Background(), source.ID(), "22222222-2222-4222-8222-222222222222")
	if err != nil {
		t.Fatalf("OrganizeSource() error = %v", err)
	}
	if checkedSpaceID != "22222222-2222-4222-8222-222222222222" || repository.updated != source {
		t.Fatalf("organize dependencies = checked:%q updated:%p", checkedSpaceID, repository.updated)
	}
	if organized.SpaceID() != checkedSpaceID || !organized.UpdatedAt().Equal(now) {
		t.Fatalf("organized source = space:%q updatedAt:%v", organized.SpaceID(), organized.UpdatedAt())
	}
}

func TestServiceRejectsUnknownTargetSpaceAndReturnsToInboxWithoutLookup(t *testing.T) {
	now := time.Date(2026, 7, 31, 9, 0, 0, 0, time.UTC)
	source, _ := NewManualSource("11111111-1111-4111-8111-111111111111", "Source", "", "", now.Add(-time.Hour))
	source.Organize("22222222-2222-4222-8222-222222222222", now.Add(-30*time.Minute))
	repository := &sourceRepositoryStub{source: source}
	lookupCount := 0
	service := NewService(ServiceConfig{
		Repository: repository,
		Now:        func() time.Time { return now },
		SpaceExists: func(context.Context, string) (bool, error) {
			lookupCount++
			return false, nil
		},
	})

	if _, err := service.OrganizeSource(context.Background(), source.ID(), "33333333-3333-4333-8333-333333333333"); !errors.Is(err, ErrSourceSpaceNotFound) {
		t.Fatalf("unknown space error = %v, want %v", err, ErrSourceSpaceNotFound)
	}
	if repository.updated != nil {
		t.Fatal("unknown space updated the source")
	}

	returned, err := service.OrganizeSource(context.Background(), source.ID(), "")
	if err != nil {
		t.Fatalf("return to inbox error = %v", err)
	}
	if lookupCount != 1 || returned.SpaceID() != "" || repository.updated != source {
		t.Fatalf("return to inbox = lookups:%d space:%q updated:%p", lookupCount, returned.SpaceID(), repository.updated)
	}
}

type sourceRepositoryStub struct {
	source  *Source
	updated *Source
}

func (repository *sourceRepositoryStub) CreateSource(context.Context, *Source) error { return nil }
func (repository *sourceRepositoryStub) GetSource(context.Context, string) (*Source, error) {
	return repository.source, nil
}
func (repository *sourceRepositoryStub) ListSources(context.Context, ListFilter) (SourcePage, error) {
	return SourcePage{}, nil
}
func (repository *sourceRepositoryStub) UpdateSourceOrganization(_ context.Context, source *Source) error {
	repository.updated = source
	return nil
}
