package sources

import (
	"errors"
	"strings"
	"time"
)

type Kind string

const (
	KindManual Kind = "manual"
	KindURL    Kind = "url"
	KindPDF    Kind = "pdf"
)

type ProcessingStatus string

const (
	ProcessingStatusPending    ProcessingStatus = "pending"
	ProcessingStatusProcessing ProcessingStatus = "processing"
	ProcessingStatusReady      ProcessingStatus = "ready"
	ProcessingStatusFailed     ProcessingStatus = "failed"
)

var (
	ErrSourceIDRequired            = errors.New("Learning Source id is required")
	ErrSourceTitleRequired         = errors.New("Learning Source title is required")
	ErrSourceNotFound              = errors.New("Learning Source not found")
	ErrSourceSpaceNotFound         = errors.New("Learning Source target Knowledge Space not found")
	ErrSourceURLInvalid            = errors.New("Learning Source URL must be an absolute HTTP or HTTPS URL without credentials")
	ErrSourceURLConflict           = errors.New("Learning Source URL already exists")
	ErrSourceExtractionUnsupported = errors.New("Learning Source kind does not support extraction")
	ErrSourceExtractionNotFailed   = errors.New("Learning Source extraction is not failed")
)

type Source struct {
	id               string
	kind             Kind
	spaceID          string
	title            string
	captureNote      string
	content          string
	originalURL      string
	normalizedURL    string
	processingStatus ProcessingStatus
	failureMessage   string
	createdAt        time.Time
	updatedAt        time.Time
}

type SourceSummary struct {
	ID               string
	Kind             Kind
	SpaceID          string
	Title            string
	CaptureNote      string
	ProcessingStatus ProcessingStatus
	CreatedAt        time.Time
	UpdatedAt        time.Time
	OriginalURL      string
}

func NewURLSource(id, title, captureNote, originalURL, normalizedURL string, now time.Time) (*Source, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, ErrSourceIDRequired
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, ErrSourceTitleRequired
	}
	originalURL = strings.TrimSpace(originalURL)
	normalizedURL = strings.TrimSpace(normalizedURL)
	if originalURL == "" || normalizedURL == "" {
		return nil, ErrSourceURLInvalid
	}
	return &Source{
		id:               id,
		kind:             KindURL,
		title:            title,
		captureNote:      strings.TrimSpace(captureNote),
		originalURL:      originalURL,
		normalizedURL:    normalizedURL,
		processingStatus: ProcessingStatusPending,
		createdAt:        now,
		updatedAt:        now,
	}, nil
}

func NewManualSource(id, title, captureNote, content string, now time.Time) (*Source, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, ErrSourceIDRequired
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, ErrSourceTitleRequired
	}
	return &Source{
		id:               id,
		kind:             KindManual,
		title:            title,
		captureNote:      strings.TrimSpace(captureNote),
		content:          content,
		processingStatus: ProcessingStatusReady,
		createdAt:        now,
		updatedAt:        now,
	}, nil
}

func (source *Source) ID() string                         { return source.id }
func (source *Source) Kind() Kind                         { return source.kind }
func (source *Source) SpaceID() string                    { return source.spaceID }
func (source *Source) Title() string                      { return source.title }
func (source *Source) CaptureNote() string                { return source.captureNote }
func (source *Source) Content() string                    { return source.content }
func (source *Source) OriginalURL() string                { return source.originalURL }
func (source *Source) NormalizedURL() string              { return source.normalizedURL }
func (source *Source) ProcessingStatus() ProcessingStatus { return source.processingStatus }
func (source *Source) FailureMessage() string             { return source.failureMessage }
func (source *Source) CreatedAt() time.Time               { return source.createdAt }
func (source *Source) UpdatedAt() time.Time               { return source.updatedAt }

func (source *Source) Organize(spaceID string, now time.Time) {
	source.spaceID = strings.TrimSpace(spaceID)
	source.updatedAt = now
}

func (source *Source) MarkExtractionProcessing(now time.Time) error {
	if source.kind != KindURL {
		return ErrSourceExtractionUnsupported
	}
	source.processingStatus = ProcessingStatusProcessing
	source.failureMessage = ""
	source.updatedAt = now
	return nil
}

func (source *Source) CompleteExtraction(title, content string, now time.Time) error {
	if source.kind != KindURL {
		return ErrSourceExtractionUnsupported
	}
	if title = strings.TrimSpace(title); title != "" {
		if runes := []rune(title); len(runes) > 240 {
			title = string(runes[:240])
		}
		source.title = title
	}
	source.content = content
	source.processingStatus = ProcessingStatusReady
	source.failureMessage = ""
	source.updatedAt = now
	return nil
}

func (source *Source) FailExtraction(message string, now time.Time) error {
	if source.kind != KindURL {
		return ErrSourceExtractionUnsupported
	}
	message = strings.TrimSpace(message)
	if message == "" {
		message = "URL extraction failed"
	}
	source.processingStatus = ProcessingStatusFailed
	source.failureMessage = message
	source.updatedAt = now
	return nil
}

func (source *Source) RetryExtraction(message string, now time.Time) error {
	if source.kind != KindURL {
		return ErrSourceExtractionUnsupported
	}
	source.processingStatus = ProcessingStatusPending
	source.failureMessage = strings.TrimSpace(message)
	source.updatedAt = now
	return nil
}

func (source *Source) Summary() SourceSummary {
	return SourceSummary{
		ID:               source.id,
		Kind:             source.kind,
		SpaceID:          source.spaceID,
		Title:            source.title,
		CaptureNote:      source.captureNote,
		ProcessingStatus: source.processingStatus,
		CreatedAt:        source.createdAt,
		UpdatedAt:        source.updatedAt,
		OriginalURL:      source.originalURL,
	}
}
