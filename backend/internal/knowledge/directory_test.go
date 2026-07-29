package knowledge

import (
	"errors"
	"testing"
)

func TestNewDirectoryCreatesNestedContainerWithinSpace(t *testing.T) {
	directory, err := NewDirectory(
		"22222222-2222-4222-8222-222222222222",
		"11111111-1111-4111-8111-111111111111",
		"33333333-3333-4333-8333-333333333333",
		"  Hermes Agent  ",
	)
	if err != nil {
		t.Fatalf("NewDirectory() error = %v", err)
	}

	if directory.ID() != "22222222-2222-4222-8222-222222222222" {
		t.Errorf("ID = %q", directory.ID())
	}
	if directory.SpaceID() != "11111111-1111-4111-8111-111111111111" {
		t.Errorf("SpaceID = %q", directory.SpaceID())
	}
	if directory.ParentID() != "33333333-3333-4333-8333-333333333333" {
		t.Errorf("ParentID = %q", directory.ParentID())
	}
	if directory.Name() != "Hermes Agent" {
		t.Errorf("Name = %q", directory.Name())
	}
}

func TestNewDirectoryRejectsSelfParent(t *testing.T) {
	_, err := NewDirectory(
		"22222222-2222-4222-8222-222222222222",
		"11111111-1111-4111-8111-111111111111",
		"22222222-2222-4222-8222-222222222222",
		"Hermes Agent",
	)
	if !errors.Is(err, ErrDirectorySelfParent) {
		t.Fatalf("NewDirectory() error = %v, want %v", err, ErrDirectorySelfParent)
	}
}

func TestDirectoryRenameAndMovePreserveStableIdentityAndSpace(t *testing.T) {
	directory, err := NewDirectory(
		"22222222-2222-4222-8222-222222222222",
		"11111111-1111-4111-8111-111111111111",
		"",
		"Hermes",
	)
	if err != nil {
		t.Fatalf("NewDirectory() error = %v", err)
	}

	if err := directory.Rename("  Hermes Agent  "); err != nil {
		t.Fatalf("Rename() error = %v", err)
	}
	if err := directory.Move("33333333-3333-4333-8333-333333333333"); err != nil {
		t.Fatalf("Move() error = %v", err)
	}

	if directory.ID() != "22222222-2222-4222-8222-222222222222" || directory.SpaceID() != "11111111-1111-4111-8111-111111111111" {
		t.Errorf("identity changed: id=%q space=%q", directory.ID(), directory.SpaceID())
	}
	if directory.Name() != "Hermes Agent" || directory.ParentID() != "33333333-3333-4333-8333-333333333333" {
		t.Errorf("directory = name:%q parent:%q", directory.Name(), directory.ParentID())
	}
}
