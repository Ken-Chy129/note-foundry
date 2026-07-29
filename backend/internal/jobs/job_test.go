package jobs

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestNewJobValidatesRequiredFields(t *testing.T) {
	now := time.Date(2026, 7, 29, 0, 0, 0, 0, time.UTC)
	if _, err := New("", "backup.create", nil, 3, now, ""); !errors.Is(err, ErrJobIDRequired) {
		t.Fatalf("New(empty id) error = %v", err)
	}
	if _, err := New("id", "", nil, 3, now, ""); !errors.Is(err, ErrJobKindRequired) {
		t.Fatalf("New(empty kind) error = %v", err)
	}
	if _, err := New("id", "backup.create", nil, 0, now, ""); !errors.Is(err, ErrInvalidMaxAttempts) {
		t.Fatalf("New(invalid attempts) error = %v", err)
	}
	if _, err := New("id", "backup.create", json.RawMessage(`{`), 3, now, ""); err == nil {
		t.Fatal("New(invalid JSON) error = nil")
	}
}
