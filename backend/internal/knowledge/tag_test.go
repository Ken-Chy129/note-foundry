package knowledge

import (
	"errors"
	"testing"
)

func TestNewTagCreatesGlobalFlatClassification(t *testing.T) {
	tag, err := NewTag("11111111-1111-4111-8111-111111111111", "  memory  ")
	if err != nil {
		t.Fatalf("NewTag() error = %v", err)
	}
	if tag.ID() != "11111111-1111-4111-8111-111111111111" || tag.Name() != "memory" {
		t.Errorf("tag = id:%q name:%q", tag.ID(), tag.Name())
	}
}

func TestTagRejectsBlankNameAndPreservesIdentityOnRename(t *testing.T) {
	tag, err := NewTag("11111111-1111-4111-8111-111111111111", "agent")
	if err != nil {
		t.Fatalf("NewTag() error = %v", err)
	}
	if err := tag.Rename("  agent-design  "); err != nil {
		t.Fatalf("Rename() error = %v", err)
	}
	if tag.ID() != "11111111-1111-4111-8111-111111111111" || tag.Name() != "agent-design" {
		t.Errorf("tag = id:%q name:%q", tag.ID(), tag.Name())
	}
	if err := tag.Rename(" "); !errors.Is(err, ErrTagNameRequired) {
		t.Fatalf("blank Rename() error = %v, want %v", err, ErrTagNameRequired)
	}
}
