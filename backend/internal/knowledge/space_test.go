package knowledge

import (
	"errors"
	"testing"
)

func TestNewSpaceCreatesFlatTopLevelContainer(t *testing.T) {
	space, err := NewSpace("space-1", "  AI Agent  ", VisibilityPublic)
	if err != nil {
		t.Fatalf("NewSpace() error = %v", err)
	}

	if got := space.ID(); got != "space-1" {
		t.Errorf("ID() = %q, want %q", got, "space-1")
	}
	if got := space.Name(); got != "AI Agent" {
		t.Errorf("Name() = %q, want %q", got, "AI Agent")
	}
	if got := space.Visibility(); got != VisibilityPublic {
		t.Errorf("Visibility() = %q, want %q", got, VisibilityPublic)
	}
}

func TestNewSpaceRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name       string
		id         string
		spaceName  string
		visibility Visibility
		wantErr    error
	}{
		{
			name:       "missing stable identity",
			id:         " ",
			spaceName:  "AI Agent",
			visibility: VisibilityPrivate,
			wantErr:    ErrSpaceIDRequired,
		},
		{
			name:       "blank name",
			id:         "space-1",
			spaceName:  " \t\n ",
			visibility: VisibilityPrivate,
			wantErr:    ErrSpaceNameRequired,
		},
		{
			name:       "unknown visibility",
			id:         "space-1",
			spaceName:  "AI Agent",
			visibility: Visibility("unlisted"),
			wantErr:    ErrInvalidSpaceVisibility,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewSpace(tt.id, tt.spaceName, tt.visibility)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("NewSpace() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestRenameSpacePreservesStableIdentity(t *testing.T) {
	space, err := NewSpace("space-1", "AI", VisibilityPrivate)
	if err != nil {
		t.Fatalf("NewSpace() error = %v", err)
	}

	if err := space.Rename("  AI Agent  "); err != nil {
		t.Fatalf("Rename() error = %v", err)
	}

	if got := space.ID(); got != "space-1" {
		t.Errorf("ID() after rename = %q, want %q", got, "space-1")
	}
	if got := space.Name(); got != "AI Agent" {
		t.Errorf("Name() after rename = %q, want %q", got, "AI Agent")
	}
}

func TestRenameSpaceRejectsBlankNameWithoutChangingCurrentName(t *testing.T) {
	space, err := NewSpace("space-1", "AI Agent", VisibilityPrivate)
	if err != nil {
		t.Fatalf("NewSpace() error = %v", err)
	}

	err = space.Rename(" ")
	if !errors.Is(err, ErrSpaceNameRequired) {
		t.Fatalf("Rename() error = %v, want %v", err, ErrSpaceNameRequired)
	}
	if got := space.Name(); got != "AI Agent" {
		t.Errorf("Name() after rejected rename = %q, want %q", got, "AI Agent")
	}
}
