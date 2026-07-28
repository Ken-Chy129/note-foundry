package attachments

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

var (
	ErrAttachmentNameRequired = errors.New("Attachment file name is required")
	ErrAttachmentEmpty        = errors.New("Attachment file is empty")
)

type Repository interface {
	NoteExists(context.Context, string) (bool, error)
	Create(context.Context, Attachment) error
	ListForNote(context.Context, string) ([]Attachment, error)
	GetOwner(context.Context, string) (Attachment, error)
	GetPublic(context.Context, string) (Attachment, error)
	ReplacePublishedNoteAttachments(context.Context, string, []string, time.Time) error
}

type Storage interface {
	Put(context.Context, string, io.Reader) (StoredObject, error)
	Open(context.Context, string) (ReadSeekCloser, error)
	Delete(context.Context, string) error
}

type ReadSeekCloser interface {
	io.Reader
	io.Seeker
	io.Closer
}

type IDGenerator func() string

type Service struct {
	repository Repository
	storage    Storage
	generateID IDGenerator
	now        func() time.Time
}

func NewService(repository Repository, storage Storage, generateID IDGenerator, now func() time.Time) *Service {
	return &Service{repository: repository, storage: storage, generateID: generateID, now: now}
}

func (service *Service) Upload(ctx context.Context, noteID, originalName string, reader io.Reader) (Attachment, error) {
	originalName = filepath.Base(strings.TrimSpace(originalName))
	originalName = strings.NewReplacer("\r", "", "\n", "").Replace(originalName)
	if originalName == "" || originalName == "." {
		return Attachment{}, ErrAttachmentNameRequired
	}
	exists, err := service.repository.NoteExists(ctx, noteID)
	if err != nil {
		return Attachment{}, fmt.Errorf("check Attachment Learning Note: %w", err)
	}
	if !exists {
		return Attachment{}, ErrAttachmentNoteNotFound
	}
	firstBytes := make([]byte, 512)
	count, readErr := io.ReadFull(reader, firstBytes)
	if errors.Is(readErr, io.EOF) && count == 0 {
		return Attachment{}, ErrAttachmentEmpty
	}
	if readErr != nil && !errors.Is(readErr, io.ErrUnexpectedEOF) && !errors.Is(readErr, io.EOF) {
		return Attachment{}, fmt.Errorf("inspect Attachment content: %w", readErr)
	}
	firstBytes = firstBytes[:count]
	mediaType := http.DetectContentType(firstBytes)
	if mediaType == "application/octet-stream" {
		if extensionType := mime.TypeByExtension(strings.ToLower(filepath.Ext(originalName))); extensionType != "" {
			mediaType = extensionType
		}
	}
	id := service.generateID()
	storageKey := service.generateID()
	stored, err := service.storage.Put(ctx, storageKey, io.MultiReader(bytes.NewReader(firstBytes), reader))
	if err != nil {
		return Attachment{}, fmt.Errorf("store Attachment: %w", err)
	}
	attachment := Attachment{
		ID:           id,
		NoteID:       noteID,
		StorageKey:   storageKey,
		OriginalName: originalName,
		MediaType:    mediaType,
		SizeBytes:    stored.SizeBytes,
		SHA256Hex:    stored.SHA256Hex,
		CreatedAt:    service.now(),
	}
	if err := service.repository.Create(ctx, attachment); err != nil {
		_ = service.storage.Delete(context.Background(), storageKey)
		return Attachment{}, fmt.Errorf("create Attachment metadata: %w", err)
	}
	return attachment, nil
}

func (service *Service) ListForNote(ctx context.Context, noteID string) ([]Attachment, error) {
	attachments, err := service.repository.ListForNote(ctx, noteID)
	if err != nil {
		return nil, fmt.Errorf("list Learning Note Attachments: %w", err)
	}
	return attachments, nil
}

func (service *Service) OpenOwner(ctx context.Context, id string) (Attachment, ReadSeekCloser, error) {
	attachment, err := service.repository.GetOwner(ctx, id)
	if err != nil {
		return Attachment{}, nil, fmt.Errorf("load owner Attachment: %w", err)
	}
	file, err := service.storage.Open(ctx, attachment.StorageKey)
	if err != nil {
		return Attachment{}, nil, fmt.Errorf("open owner Attachment content: %w", err)
	}
	return attachment, file, nil
}

func (service *Service) OpenPublic(ctx context.Context, id string) (Attachment, ReadSeekCloser, error) {
	attachment, err := service.repository.GetPublic(ctx, id)
	if err != nil {
		return Attachment{}, nil, fmt.Errorf("load public Attachment: %w", err)
	}
	file, err := service.storage.Open(ctx, attachment.StorageKey)
	if err != nil {
		return Attachment{}, nil, fmt.Errorf("open public Attachment content: %w", err)
	}
	return attachment, file, nil
}

func (service *Service) ReplacePublishedNoteAttachments(ctx context.Context, noteID string, attachmentIDs []string, publishedAt time.Time) error {
	if err := service.repository.ReplacePublishedNoteAttachments(ctx, noteID, attachmentIDs, publishedAt); err != nil {
		return fmt.Errorf("replace published Learning Note Attachments: %w", err)
	}
	return nil
}
