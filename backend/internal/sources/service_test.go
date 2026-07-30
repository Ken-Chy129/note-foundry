package sources

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestServiceOrganizesSourceIntoExistingSpace(t *testing.T) {
	now := time.Date(2026, 7, 31, 9, 0, 0, 0, time.UTC)
	source, _ := NewManualSource("11111111-1111-4111-8111-111111111111", "Source", "", "", now.Add(-time.Hour))
	repository := &sourceRepositoryStub{source: source}
	checkedSpaceID := ""
	service := NewService(ServiceConfig{
		Repository: repository,
		Now:        func() time.Time { return now },
		SpaceExists: func(_ context.Context, spaceID string) (bool, error) {
			checkedSpaceID = spaceID
			return true, nil
		},
	})

	organized, err := service.OrganizeSource(context.Background(), source.ID(), "22222222-2222-4222-8222-222222222222")
	if err != nil {
		t.Fatalf("OrganizeSource() error = %v", err)
	}
	if checkedSpaceID != "22222222-2222-4222-8222-222222222222" || repository.updated != source {
		t.Fatalf("organize dependencies = checked:%q updated:%p", checkedSpaceID, repository.updated)
	}
	if organized.SpaceID() != checkedSpaceID || !organized.UpdatedAt().Equal(now) {
		t.Fatalf("organized source = space:%q updatedAt:%v", organized.SpaceID(), organized.UpdatedAt())
	}
}

func TestServiceRejectsUnknownTargetSpaceAndReturnsToInboxWithoutLookup(t *testing.T) {
	now := time.Date(2026, 7, 31, 9, 0, 0, 0, time.UTC)
	source, _ := NewManualSource("11111111-1111-4111-8111-111111111111", "Source", "", "", now.Add(-time.Hour))
	source.Organize("22222222-2222-4222-8222-222222222222", now.Add(-30*time.Minute))
	repository := &sourceRepositoryStub{source: source}
	lookupCount := 0
	service := NewService(ServiceConfig{
		Repository: repository,
		Now:        func() time.Time { return now },
		SpaceExists: func(context.Context, string) (bool, error) {
			lookupCount++
			return false, nil
		},
	})

	if _, err := service.OrganizeSource(context.Background(), source.ID(), "33333333-3333-4333-8333-333333333333"); !errors.Is(err, ErrSourceSpaceNotFound) {
		t.Fatalf("unknown space error = %v, want %v", err, ErrSourceSpaceNotFound)
	}
	if repository.updated != nil {
		t.Fatal("unknown space updated the source")
	}

	returned, err := service.OrganizeSource(context.Background(), source.ID(), "")
	if err != nil {
		t.Fatalf("return to inbox error = %v", err)
	}
	if lookupCount != 1 || returned.SpaceID() != "" || repository.updated != source {
		t.Fatalf("return to inbox = lookups:%d space:%q updated:%p", lookupCount, returned.SpaceID(), repository.updated)
	}
}

func TestServiceCapturesAndDeduplicatesURLSources(t *testing.T) {
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	repository := &sourceRepositoryStub{}
	queue := &urlExtractionQueueStub{}
	service := NewService(ServiceConfig{
		Repository:    repository,
		GenerateID:    func() string { return "11111111-1111-4111-8111-111111111111" },
		Now:           func() time.Time { return now },
		URLExtraction: queue,
	})

	result, err := service.CreateURLSource(context.Background(), CreateURLSourceInput{
		OriginalURL: "HTTPS://Example.COM:443/docs?b=2&a=1#top",
		CaptureNote: "Read later",
	})
	if err != nil {
		t.Fatalf("CreateURLSource() error = %v", err)
	}
	if !result.Created || result.Source.Title() != "example.com" || result.Source.NormalizedURL() != "https://example.com/docs?a=1&b=2" {
		t.Fatalf("created URL result = %+v source:%+v", result, result.Source)
	}
	if repository.created != result.Source || queue.sourceID != result.Source.ID() || !queue.sourceVersion.Equal(now) {
		t.Fatalf("capture dependencies = created:%p queued:%q version:%v", repository.created, queue.sourceID, queue.sourceVersion)
	}

	repository.existing = result.Source
	repository.created = nil
	queue.sourceID = ""
	duplicate, err := service.CreateURLSource(context.Background(), CreateURLSourceInput{OriginalURL: "https://example.com/docs?a=1&b=2"})
	if err != nil {
		t.Fatalf("duplicate CreateURLSource() error = %v", err)
	}
	if duplicate.Created || duplicate.Source != result.Source || repository.created != nil || queue.sourceID != "" {
		t.Fatalf("duplicate URL result = %+v created:%p queued:%q", duplicate, repository.created, queue.sourceID)
	}

	_ = result.Source.CompleteExtraction("Article", "Body", now.Add(time.Minute))
	queue.sourceID = ""
	if _, err := service.CreateURLSource(context.Background(), CreateURLSourceInput{OriginalURL: result.Source.NormalizedURL()}); err != nil {
		t.Fatalf("ready duplicate CreateURLSource() error = %v", err)
	}
	if queue.sourceID != "" {
		t.Fatalf("ready duplicate unexpectedly queued extraction for %q", queue.sourceID)
	}

	failedAt := now.Add(2 * time.Minute)
	_ = result.Source.FailExtraction("final failure", failedAt)
	if _, err := service.CreateURLSource(context.Background(), CreateURLSourceInput{OriginalURL: result.Source.NormalizedURL()}); err != nil {
		t.Fatalf("failed duplicate CreateURLSource() error = %v", err)
	}
	if queue.sourceID != result.Source.ID() || !queue.sourceVersion.Equal(failedAt) {
		t.Fatalf("failed duplicate retry = source:%q version:%v", queue.sourceID, queue.sourceVersion)
	}
}

func TestServiceRetriesFailedURLExtraction(t *testing.T) {
	failedAt := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	retriedAt := failedAt.Add(time.Hour)
	source, _ := NewURLSource("11111111-1111-4111-8111-111111111111", "example.com", "", "https://example.com/", "https://example.com/", failedAt.Add(-time.Hour))
	_ = source.FailExtraction("timeout", failedAt)
	repository := &sourceRepositoryStub{source: source}
	queue := &urlExtractionQueueStub{}
	service := NewService(ServiceConfig{Repository: repository, URLExtraction: queue, Now: func() time.Time { return retriedAt }})

	retried, err := service.RetryURLExtraction(context.Background(), source.ID())
	if err != nil {
		t.Fatalf("RetryURLExtraction() error = %v", err)
	}
	if retried.ProcessingStatus() != ProcessingStatusPending || repository.updated != source {
		t.Fatalf("retried source = status:%q updated:%p", retried.ProcessingStatus(), repository.updated)
	}
	if queue.sourceID != source.ID() || !queue.sourceVersion.Equal(failedAt) {
		t.Fatalf("retry queue = source:%q version:%v", queue.sourceID, queue.sourceVersion)
	}
}

type sourceRepositoryStub struct {
	source   *Source
	updated  *Source
	created  *Source
	existing *Source
}

func (repository *sourceRepositoryStub) CreateSource(_ context.Context, source *Source) error {
	repository.created = source
	return nil
}
func (repository *sourceRepositoryStub) GetSource(context.Context, string) (*Source, error) {
	return repository.source, nil
}
func (repository *sourceRepositoryStub) ListSources(context.Context, ListFilter) (SourcePage, error) {
	return SourcePage{}, nil
}
func (repository *sourceRepositoryStub) UpdateSourceOrganization(_ context.Context, source *Source) error {
	repository.updated = source
	return nil
}

func (repository *sourceRepositoryStub) UpdateSourceExtraction(_ context.Context, source *Source) error {
	repository.updated = source
	return nil
}

func (repository *sourceRepositoryStub) FindSourceByNormalizedURL(context.Context, string) (*Source, error) {
	if repository.existing == nil {
		return nil, ErrSourceNotFound
	}
	return repository.existing, nil
}

type urlExtractionQueueStub struct {
	sourceID      string
	sourceVersion time.Time
}

func (queue *urlExtractionQueueStub) EnsureURLExtraction(_ context.Context, sourceID string, sourceVersion time.Time) error {
	queue.sourceID = sourceID
	queue.sourceVersion = sourceVersion
	return nil
}
