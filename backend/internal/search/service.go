package search

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var ErrQueryRequired = errors.New("search query is required")

type Options struct {
	Query    string
	SpaceID  string
	TagID    string
	Page     int
	PageSize int
}

type Result struct {
	ID      string
	SpaceID string
	Title   string
	Slug    string
	Snippet string
	Rank    float64
}

type Page struct {
	Results    []Result
	Page       int
	PageSize   int
	TotalItems int
}

type Repository interface {
	UpsertCurrentNote(context.Context, string, string, string, string) error
	UpsertPublishedNote(context.Context, string, string, string, string) error
	RefreshNoteTags(context.Context, string) error
	RefreshNotesForTag(context.Context, string) error
	SearchOwner(context.Context, Options) (Page, error)
	SearchPublic(context.Context, Options) (Page, error)
}

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (service *Service) ProjectCurrentNote(ctx context.Context, noteID, title, markdown string) error {
	if err := service.repository.UpsertCurrentNote(ctx, noteID, title, ExtractMarkdownHeadings(markdown), markdown); err != nil {
		return fmt.Errorf("project current Learning Note search document: %w", err)
	}
	return nil
}

func (service *Service) ProjectPublishedNote(ctx context.Context, noteID, title, markdown string) error {
	if err := service.repository.UpsertPublishedNote(ctx, noteID, title, ExtractMarkdownHeadings(markdown), markdown); err != nil {
		return fmt.Errorf("project published Learning Note search document: %w", err)
	}
	return nil
}

func (service *Service) RefreshNoteTags(ctx context.Context, noteID string) error {
	if err := service.repository.RefreshNoteTags(ctx, noteID); err != nil {
		return fmt.Errorf("refresh Learning Note Tag search projection: %w", err)
	}
	return nil
}

func (service *Service) RefreshNotesForTag(ctx context.Context, tagID string) error {
	if err := service.repository.RefreshNotesForTag(ctx, tagID); err != nil {
		return fmt.Errorf("refresh Tag search projections: %w", err)
	}
	return nil
}

func (service *Service) SearchOwner(ctx context.Context, options Options) (Page, error) {
	options.Query = strings.TrimSpace(options.Query)
	if options.Query == "" {
		return Page{}, ErrQueryRequired
	}
	page, err := service.repository.SearchOwner(ctx, options)
	if err != nil {
		return Page{}, fmt.Errorf("search owner Learning Notes: %w", err)
	}
	return page, nil
}

func (service *Service) SearchPublic(ctx context.Context, options Options) (Page, error) {
	options.Query = strings.TrimSpace(options.Query)
	if options.Query == "" {
		return Page{}, ErrQueryRequired
	}
	page, err := service.repository.SearchPublic(ctx, options)
	if err != nil {
		return Page{}, fmt.Errorf("search public Learning Notes: %w", err)
	}
	return page, nil
}
