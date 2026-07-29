package knowledge

import (
	"errors"
	"strings"
)

var (
	ErrSpaceIDRequired        = errors.New("knowledge space id is required")
	ErrSpaceNameRequired      = errors.New("knowledge space name is required")
	ErrInvalidSpaceVisibility = errors.New("knowledge space visibility must be private or public")
)

type Visibility string

const (
	VisibilityPrivate Visibility = "private"
	VisibilityPublic  Visibility = "public"
)

type Space struct {
	id         string
	name       string
	visibility Visibility
}

func NewSpace(id, name string, visibility Visibility) (*Space, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, ErrSpaceIDRequired
	}

	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrSpaceNameRequired
	}

	if visibility != VisibilityPrivate && visibility != VisibilityPublic {
		return nil, ErrInvalidSpaceVisibility
	}

	return &Space{
		id:         id,
		name:       name,
		visibility: visibility,
	}, nil
}

func (s *Space) ID() string {
	return s.id
}

func (s *Space) Name() string {
	return s.name
}

func (s *Space) Visibility() Visibility {
	return s.visibility
}

func (s *Space) Rename(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return ErrSpaceNameRequired
	}

	s.name = name
	return nil
}
