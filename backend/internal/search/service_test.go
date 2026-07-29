package search

import (
	"context"
	"errors"
	"testing"
)

func TestServiceProjectsHeadingsAndValidatesSearchQuery(t *testing.T) {
	repository := &repositoryStub{page: Page{Results: []Result{{ID: "note-1"}}, Page: 1, PageSize: 20, TotalItems: 1}}
	service := NewService(repository)
	if err := service.ProjectCurrentNote(context.Background(), "note-1", "Agent Loop", "# Architecture\nbody"); err != nil {
		t.Fatalf("ProjectCurrentNote() error = %v", err)
	}
	if repository.headings != "Architecture" {
		t.Errorf("headings = %q", repository.headings)
	}
	if _, err := service.SearchOwner(context.Background(), Options{Query: "  ", Page: 1, PageSize: 20}); !errors.Is(err, ErrQueryRequired) {
		t.Fatalf("SearchOwner(empty) error = %v, want %v", err, ErrQueryRequired)
	}
	page, err := service.SearchPublic(context.Background(), Options{Query: "memory", Page: 1, PageSize: 20})
	if err != nil || page.TotalItems != 1 {
		t.Fatalf("SearchPublic() = %+v, %v", page, err)
	}
}

type repositoryStub struct {
	headings string
	page     Page
}

func (repository *repositoryStub) UpsertCurrentNote(_ context.Context, _, _, headings, _ string) error {
	repository.headings = headings
	return nil
}

func (repository *repositoryStub) UpsertPublishedNote(context.Context, string, string, string, string) error {
	return nil
}

func (repository *repositoryStub) RefreshNoteTags(context.Context, string) error {
	return nil
}

func (repository *repositoryStub) RefreshNotesForTag(context.Context, string) error {
	return nil
}

func (repository *repositoryStub) SearchOwner(context.Context, Options) (Page, error) {
	return repository.page, nil
}

func (repository *repositoryStub) SearchPublic(context.Context, Options) (Page, error) {
	return repository.page, nil
}
