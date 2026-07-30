package sources

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Repository interface {
	CreateSource(context.Context, *Source) error
	GetSource(context.Context, string) (*Source, error)
	ListSources(context.Context, ListFilter) (SourcePage, error)
	UpdateSourceOrganization(context.Context, *Source) error
}

type IDGenerator func() string

type Service struct {
	repository  Repository
	generateID  IDGenerator
	now         func() time.Time
	spaceExists func(context.Context, string) (bool, error)
}

type ServiceConfig struct {
	Repository  Repository
	GenerateID  IDGenerator
	Now         func() time.Time
	SpaceExists func(context.Context, string) (bool, error)
}

type CreateManualSourceInput struct {
	Title       string
	CaptureNote string
	Content     string
}

type ListFilter struct {
	InboxOnly bool
	SpaceID   string
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
	return &Service{repository: config.Repository, generateID: config.GenerateID, now: config.Now, spaceExists: config.SpaceExists}
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

func (service *Service) OrganizeSource(ctx context.Context, id, spaceID string) (*Source, error) {
	spaceID = strings.TrimSpace(spaceID)
	source, err := service.repository.GetSource(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("load Learning Source: %w", err)
	}
	if spaceID != "" {
		if service.spaceExists == nil {
			return nil, errors.New("Learning Source space lookup is not configured")
		}
		exists, err := service.spaceExists(ctx, spaceID)
		if err != nil {
			return nil, fmt.Errorf("check target Knowledge Space: %w", err)
		}
		if !exists {
			return nil, ErrSourceSpaceNotFound
		}
	}
	source.Organize(spaceID, service.now())
	if err := service.repository.UpdateSourceOrganization(ctx, source); err != nil {
		return nil, fmt.Errorf("organize Learning Source: %w", err)
	}
	return source, nil
}
