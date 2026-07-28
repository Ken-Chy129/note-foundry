package attachments

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestLocalStorageUsesValidatedImmutableKeyAndChecksum(t *testing.T) {
	storage, err := NewLocalStorage(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalStorage() error = %v", err)
	}
	key := "11111111-1111-4111-8111-111111111111"
	stored, err := storage.Put(context.Background(), key, strings.NewReader("diagram"))
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	if stored.SizeBytes != 7 || stored.SHA256Hex != "2c20a6b064ee805c68638fcdf13455bb59c90d4293528c22437afd44dd7027fd" {
		t.Errorf("stored object = %+v", stored)
	}
	file, err := storage.Open(context.Background(), key)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer file.Close()
	content, _ := io.ReadAll(file)
	if string(content) != "diagram" {
		t.Errorf("content = %q", content)
	}
	if _, err := storage.Open(context.Background(), "../../secret"); !errors.Is(err, ErrInvalidStorageKey) {
		t.Fatalf("Open(path traversal) error = %v, want %v", err, ErrInvalidStorageKey)
	}
}
