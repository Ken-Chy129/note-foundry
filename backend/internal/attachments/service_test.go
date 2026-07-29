package attachments

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

func TestServiceUploadsAttachmentWithGeneratedKeysAndSanitizedName(t *testing.T) {
	repository := &attachmentRepositoryStub{noteExists: true}
	storage := &storageStub{}
	ids := []string{"11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222"}
	index := 0
	service := NewService(repository, storage, func() string {
		id := ids[index]
		index++
		return id
	}, func() time.Time { return time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC) })

	attachment, err := service.Upload(context.Background(), "33333333-3333-4333-8333-333333333333", "../../diagram.png", strings.NewReader("png bytes"))
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	if attachment.OriginalName != "diagram.png" || attachment.StorageKey != ids[1] || repository.created.ID != ids[0] {
		t.Errorf("Attachment = %+v, created = %+v", attachment, repository.created)
	}
	if string(storage.content) != "png bytes" {
		t.Errorf("stored content = %q", storage.content)
	}
}

type attachmentRepositoryStub struct {
	noteExists bool
	created    Attachment
}

func (repository *attachmentRepositoryStub) NoteExists(context.Context, string) (bool, error) {
	return repository.noteExists, nil
}

func (repository *attachmentRepositoryStub) Create(_ context.Context, attachment Attachment) error {
	repository.created = attachment
	return nil
}

func (repository *attachmentRepositoryStub) ListForNote(context.Context, string) ([]Attachment, error) {
	if repository.created.ID == "" {
		return nil, nil
	}
	return []Attachment{repository.created}, nil
}

func (repository *attachmentRepositoryStub) GetOwner(context.Context, string) (Attachment, error) {
	return repository.created, nil
}

func (repository *attachmentRepositoryStub) GetPublic(context.Context, string) (Attachment, error) {
	return repository.created, nil
}

func (repository *attachmentRepositoryStub) ReplacePublishedNoteAttachments(context.Context, string, []string, time.Time) error {
	return nil
}

type storageStub struct {
	content []byte
}

func (storage *storageStub) Put(_ context.Context, _ string, reader io.Reader) (StoredObject, error) {
	storage.content, _ = io.ReadAll(reader)
	return StoredObject{SizeBytes: int64(len(storage.content)), SHA256Hex: strings.Repeat("a", 64)}, nil
}

func (storage *storageStub) Open(context.Context, string) (ReadSeekCloser, error) {
	return &memoryFile{Reader: bytes.NewReader(storage.content)}, nil
}

func (storage *storageStub) Delete(context.Context, string) error {
	return nil
}

type memoryFile struct {
	*bytes.Reader
}

func (file *memoryFile) Close() error { return nil }
