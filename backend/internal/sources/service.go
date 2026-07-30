package sources

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Repository interface {
	CreateSource(context.Context, *Source) error
	GetSource(context.Context, string) (*Source, error)
	ListSources(context.Context, ListFilter) (SourcePage, error)
	UpdateSourceOrganization(context.Context, *Source) error
	UpdateSourceExtraction(context.Context, *Source) error
	FindSourceByNormalizedURL(context.Context, string) (*Source, error)
}

type IDGenerator func() string

type Service struct {
	repository    Repository
	generateID    IDGenerator
	now           func() time.Time
	spaceExists   func(context.Context, string) (bool, error)
	urlExtraction URLExtractionQueue
}

type ServiceConfig struct {
	Repository    Repository
	GenerateID    IDGenerator
	Now           func() time.Time
	SpaceExists   func(context.Context, string) (bool, error)
	URLExtraction URLExtractionQueue
}

type CreateManualSourceInput struct {
	Title       string
	CaptureNote string
	Content     string
}

type CreateURLSourceInput struct {
	Title       string
	CaptureNote string
	OriginalURL string
}

type CreateURLSourceResult struct {
	Source  *Source
	Created bool
}

type URLExtractionQueue interface {
	EnsureURLExtraction(context.Context, string, time.Time) error
}

type ListFilter struct {
	InboxOnly bool
	SpaceID   string
	Page      int
	PageSize  int
}

type SourcePage struct {
	Sources    []SourceSummary
	Page       int
	PageSize   int
	TotalItems int
}

func NewService(config ServiceConfig) *Service {
	return &Service{repository: config.Repository, generateID: config.GenerateID, now: config.Now, spaceExists: config.SpaceExists, urlExtraction: config.URLExtraction}
}

func (service *Service) CreateURLSource(ctx context.Context, input CreateURLSourceInput) (CreateURLSourceResult, error) {
	normalizedURL, err := NormalizeSourceURL(input.OriginalURL)
	if err != nil {
		return CreateURLSourceResult{}, err
	}
	existing, err := service.repository.FindSourceByNormalizedURL(ctx, normalizedURL)
	if err == nil {
		if err := service.ensureExistingURLExtraction(ctx, existing); err != nil {
			return CreateURLSourceResult{}, err
		}
		return CreateURLSourceResult{Source: existing}, nil
	}
	if !errors.Is(err, ErrSourceNotFound) {
		return CreateURLSourceResult{}, fmt.Errorf("find duplicate Learning Source URL: %w", err)
	}
	title := strings.TrimSpace(input.Title)
	if title == "" {
		title = defaultURLSourceTitle(normalizedURL)
	}
	source, err := NewURLSource(service.generateID(), title, input.CaptureNote, strings.TrimSpace(input.OriginalURL), normalizedURL, service.now())
	if err != nil {
		return CreateURLSourceResult{}, err
	}
	if err := service.repository.CreateSource(ctx, source); err != nil {
		if errors.Is(err, ErrSourceURLConflict) {
			existing, loadErr := service.repository.FindSourceByNormalizedURL(ctx, normalizedURL)
			if loadErr != nil {
				return CreateURLSourceResult{}, fmt.Errorf("load concurrent Learning Source URL: %w", loadErr)
			}
			if err := service.ensureExistingURLExtraction(ctx, existing); err != nil {
				return CreateURLSourceResult{}, err
			}
			return CreateURLSourceResult{Source: existing}, nil
		}
		return CreateURLSourceResult{}, fmt.Errorf("create URL Learning Source: %w", err)
	}
	if err := service.enqueueURLExtraction(ctx, source.ID(), source.UpdatedAt()); err != nil {
		_ = source.FailExtraction("URL extraction could not be queued", service.now())
		if updateErr := service.repository.UpdateSourceExtraction(ctx, source); updateErr != nil {
			return CreateURLSourceResult{}, errors.Join(err, fmt.Errorf("record URL extraction queue failure: %w", updateErr))
		}
		return CreateURLSourceResult{}, err
	}
	return CreateURLSourceResult{Source: source, Created: true}, nil
}

func (service *Service) ensureExistingURLExtraction(ctx context.Context, source *Source) error {
	if source.ProcessingStatus() == ProcessingStatusFailed {
		return service.retryURLExtractionSource(ctx, source)
	}
	return nil
}

func (service *Service) enqueueURLExtraction(ctx context.Context, sourceID string, sourceVersion time.Time) error {
	if service.urlExtraction == nil {
		return errors.New("Learning Source URL extraction queue is not configured")
	}
	if err := service.urlExtraction.EnsureURLExtraction(ctx, sourceID, sourceVersion); err != nil {
		return fmt.Errorf("enqueue Learning Source URL extraction: %w", err)
	}
	return nil
}

func (service *Service) CreateManualSource(ctx context.Context, input CreateManualSourceInput) (*Source, error) {
	source, err := NewManualSource(service.generateID(), input.Title, input.CaptureNote, input.Content, service.now())
	if err != nil {
		return nil, err
	}
	if err := service.repository.CreateSource(ctx, source); err != nil {
		return nil, fmt.Errorf("create Learning Source: %w", err)
	}
	return source, nil
}

func (service *Service) GetSource(ctx context.Context, id string) (*Source, error) {
	source, err := service.repository.GetSource(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("load Learning Source: %w", err)
	}
	return source, nil
}

func (service *Service) RetryURLExtraction(ctx context.Context, id string) (*Source, error) {
	source, err := service.repository.GetSource(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("load Learning Source: %w", err)
	}
	if source.Kind() != KindURL {
		return nil, ErrSourceExtractionUnsupported
	}
	if source.ProcessingStatus() != ProcessingStatusFailed {
		return nil, ErrSourceExtractionNotFailed
	}
	if err := service.retryURLExtractionSource(ctx, source); err != nil {
		return nil, err
	}
	return source, nil
}

func (service *Service) retryURLExtractionSource(ctx context.Context, source *Source) error {
	failedVersion := source.UpdatedAt()
	failureMessage := source.FailureMessage()
	if err := source.RetryExtraction("", service.now()); err != nil {
		return err
	}
	if err := service.repository.UpdateSourceExtraction(ctx, source); err != nil {
		return fmt.Errorf("mark Learning Source extraction pending: %w", err)
	}
	if err := service.enqueueURLExtraction(ctx, source.ID(), failedVersion); err != nil {
		_ = source.FailExtraction(failureMessage, service.now())
		if updateErr := service.repository.UpdateSourceExtraction(ctx, source); updateErr != nil {
			return errors.Join(err, fmt.Errorf("restore failed Learning Source extraction: %w", updateErr))
		}
		return err
	}
	return nil
}

func (service *Service) ListSources(ctx context.Context, filter ListFilter) (SourcePage, error) {
	page, err := service.repository.ListSources(ctx, filter)
	if err != nil {
		return SourcePage{}, fmt.Errorf("list Learning Sources: %w", err)
	}
	return page, nil
}

func (service *Service) OrganizeSource(ctx context.Context, id, spaceID string) (*Source, error) {
	spaceID = strings.TrimSpace(spaceID)
	source, err := service.repository.GetSource(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("load Learning Source: %w", err)
	}
	if spaceID != "" {
		if service.spaceExists == nil {
			return nil, errors.New("Learning Source space lookup is not configured")
		}
		exists, err := service.spaceExists(ctx, spaceID)
		if err != nil {
			return nil, fmt.Errorf("check target Knowledge Space: %w", err)
		}
		if !exists {
			return nil, ErrSourceSpaceNotFound
		}
	}
	source.Organize(spaceID, service.now())
	if err := service.repository.UpdateSourceOrganization(ctx, source); err != nil {
		return nil, fmt.Errorf("organize Learning Source: %w", err)
	}
	return source, nil
}
