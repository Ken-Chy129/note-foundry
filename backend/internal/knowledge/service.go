package knowledge

import (
	"context"
	"fmt"
)

type SpaceRepository interface {
	CreateSpace(context.Context, *Space) error
	ListSpaces(context.Context, int, int) ([]*Space, int, error)
	GetSpace(context.Context, string) (*Space, error)
	UpdateSpace(context.Context, *Space) error
}

type IDGenerator func() string

type ServiceConfig struct {
	Spaces     SpaceRepository
	GenerateID IDGenerator
}

type Service struct {
	spaces     SpaceRepository
	generateID IDGenerator
}

type SpacePage struct {
	Spaces     []*Space
	Page       int
	PageSize   int
	TotalItems int
}

func NewService(config ServiceConfig) *Service {
	return &Service{spaces: config.Spaces, generateID: config.GenerateID}
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
