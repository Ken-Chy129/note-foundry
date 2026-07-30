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
	ErrSourceIDRequired    = errors.New("Learning Source id is required")
	ErrSourceTitleRequired = errors.New("Learning Source title is required")
	ErrSourceNotFound      = errors.New("Learning Source not found")
)

type Source struct {
	id               string
	kind             Kind
	spaceID          string
	title            string
	captureNote      string
	content          string
	processingStatus ProcessingStatus
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
func (source *Source) ProcessingStatus() ProcessingStatus { return source.processingStatus }
func (source *Source) CreatedAt() time.Time               { return source.createdAt }
func (source *Source) UpdatedAt() time.Time               { return source.updatedAt }

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
	}
}
