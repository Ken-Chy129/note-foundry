package knowledge

import (
	"errors"
	"strings"
)

var (
	ErrTagIDRequired   = errors.New("tag id is required")
	ErrTagNameRequired = errors.New("tag name is required")
)

type Tag struct {
	id   string
	name string
}

func NewTag(id, name string) (*Tag, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, ErrTagIDRequired
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrTagNameRequired
	}
	return &Tag{id: id, name: name}, nil
}

func (tag *Tag) ID() string {
	return tag.id
}

func (tag *Tag) Name() string {
	return tag.name
}

func (tag *Tag) Rename(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return ErrTagNameRequired
	}
	tag.name = name
	return nil
}
