package knowledge

import (
	"errors"
	"strings"
)

var (
	ErrDirectoryIDRequired      = errors.New("directory id is required")
	ErrDirectorySpaceIDRequired = errors.New("directory Knowledge Space id is required")
	ErrDirectoryNameRequired    = errors.New("directory name is required")
	ErrDirectorySelfParent      = errors.New("directory cannot be its own parent")
)

type Directory struct {
	id       string
	spaceID  string
	parentID string
	name     string
}

func NewDirectory(id, spaceID, parentID, name string) (*Directory, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, ErrDirectoryIDRequired
	}
	spaceID = strings.TrimSpace(spaceID)
	if spaceID == "" {
		return nil, ErrDirectorySpaceIDRequired
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrDirectoryNameRequired
	}
	parentID = strings.TrimSpace(parentID)
	if parentID == id {
		return nil, ErrDirectorySelfParent
	}
	return &Directory{id: id, spaceID: spaceID, parentID: parentID, name: name}, nil
}

func (directory *Directory) ID() string {
	return directory.id
}

func (directory *Directory) SpaceID() string {
	return directory.spaceID
}

func (directory *Directory) ParentID() string {
	return directory.parentID
}

func (directory *Directory) Name() string {
	return directory.name
}

func (directory *Directory) Rename(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return ErrDirectoryNameRequired
	}
	directory.name = name
	return nil
}

func (directory *Directory) Move(parentID string) error {
	parentID = strings.TrimSpace(parentID)
	if parentID == directory.id {
		return ErrDirectorySelfParent
	}
	directory.parentID = parentID
	return nil
}
