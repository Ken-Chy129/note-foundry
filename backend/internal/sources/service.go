package sources

import (
	"context"
	"fmt"
	"time"
)

type Repository interface {
	CreateSource(context.Context, *Source) error
	GetSource(context.Context, string) (*Source, error)
	ListSources(context.Context, ListFilter) (SourcePage, error)
}

type IDGenerator func() string

type Service struct {
	repository Repository
	generateID IDGenerator
	now        func() time.Time
}

type ServiceConfig struct {
	Repository Repository
	GenerateID IDGenerator
	Now        func() time.Time
}

type CreateManualSourceInput struct {
	Title       string
	CaptureNote string
	Content     string
}

type ListFilter struct {
	InboxOnly bool
	Page      int
	PageSize  int
}

type SourcePage struct {
	Sources    []SourceSummary
	Page       int
	PageSize   int
	TotalItems int
}

func NewService(config ServiceConfig) *Service {
	return &Service{repository: config.Repository, generateID: config.GenerateID, now: config.Now}
}

func (service *Service) CreateManualSource(ctx context.Context, input CreateManualSourceInput) (*Source, error) {
	source, err := NewManualSource(service.generateID(), input.Title, input.CaptureNote, input.Content, service.now())
	if err != nil {
		return nil, err
	}
	if err := service.repository.CreateSource(ctx, source); err != nil {
		return nil, fmt.Errorf("create Learning Source: %w", err)
	}
	return source, nil
}

func (service *Service) GetSource(ctx context.Context, id string) (*Source, error) {
	source, err := service.repository.GetSource(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("load Learning Source: %w", err)
	}
	return source, nil
}

func (service *Service) ListSources(ctx context.Context, filter ListFilter) (SourcePage, error) {
	page, err := service.repository.ListSources(ctx, filter)
	if err != nil {
		return SourcePage{}, fmt.Errorf("list Learning Sources: %w", err)
	}
	return page, nil
}
