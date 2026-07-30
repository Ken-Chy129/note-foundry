package sources

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Ken-Chy129/note-foundry/backend/internal/jobs"
)

func TestURLExtractionHandlerStoresExtractedDocument(t *testing.T) {
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	source, _ := NewURLSource("11111111-1111-4111-8111-111111111111", "example.com", "", "https://example.com/article", "https://example.com/article", now.Add(-time.Hour))
	repository := &urlExtractionRepositoryStub{source: source}
	extractor := &urlExtractorStub{document: ExtractedURLDocument{Title: "Article title", Content: "Article body"}}
	handler := NewURLExtractionHandler(repository, extractor, func() time.Time { return now })
	job := urlExtractionJobForTest(source)

	if err := handler.Handle(context.Background(), job); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if repository.updateCount != 2 || source.ProcessingStatus() != ProcessingStatusReady || source.Content() != "Article body" {
		t.Fatalf("stored extraction = updates:%d status:%q content:%q", repository.updateCount, source.ProcessingStatus(), source.Content())
	}
}

func TestURLExtractionHandlerPreservesOwnerTitle(t *testing.T) {
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	source, _ := NewURLSource("11111111-1111-4111-8111-111111111111", "My reading label", "", "https://example.com/article", "https://example.com/article", now.Add(-time.Hour))
	repository := &urlExtractionRepositoryStub{source: source}
	handler := NewURLExtractionHandler(repository, &urlExtractorStub{document: ExtractedURLDocument{Title: "Website title", Content: "Body"}}, func() time.Time { return now })

	if err := handler.Handle(context.Background(), urlExtractionJobForTest(source)); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if source.Title() != "My reading label" {
		t.Fatalf("source title = %q", source.Title())
	}
}

func TestURLExtractionHandlerRecordsPendingFailureForRetry(t *testing.T) {
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	source, _ := NewURLSource("11111111-1111-4111-8111-111111111111", "example.com", "", "https://example.com/article", "https://example.com/article", now.Add(-time.Hour))
	repository := &urlExtractionRepositoryStub{source: source}
	extractor := &urlExtractorStub{err: errors.New("fetch timeout")}
	handler := NewURLExtractionHandler(repository, extractor, func() time.Time { return now })

	if err := handler.Handle(context.Background(), urlExtractionJobForTest(source)); err == nil {
		t.Fatal("Handle() error = nil, want extraction failure")
	}
	if repository.updateCount != 2 || source.ProcessingStatus() != ProcessingStatusPending || source.FailureMessage() != "fetch timeout" {
		t.Fatalf("failed extraction = updates:%d status:%q failure:%q", repository.updateCount, source.ProcessingStatus(), source.FailureMessage())
	}
}

func TestURLExtractionHandlerRecordsFailureAfterFinalAttempt(t *testing.T) {
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	source, _ := NewURLSource("11111111-1111-4111-8111-111111111111", "example.com", "", "https://example.com/article", "https://example.com/article", now.Add(-time.Hour))
	repository := &urlExtractionRepositoryStub{source: source}
	handler := NewURLExtractionHandler(repository, &urlExtractorStub{err: errors.New("fetch timeout")}, func() time.Time { return now })
	job := urlExtractionJobForTest(source)
	job.Attempts = job.MaxAttempts

	if err := handler.Handle(context.Background(), job); err == nil {
		t.Fatal("Handle() error = nil, want extraction failure")
	}
	if source.ProcessingStatus() != ProcessingStatusFailed || source.FailureMessage() != "fetch timeout" {
		t.Fatalf("final extraction = status:%q failure:%q", source.ProcessingStatus(), source.FailureMessage())
	}
}

func urlExtractionJobForTest(source *Source) jobs.Job {
	scheduler := NewURLExtractionScheduler(&sourceJobEnqueuerStub{}, func() string { return "22222222-2222-4222-8222-222222222222" }, time.Now)
	enqueuer := &sourceJobEnqueuerStub{}
	scheduler.jobs = enqueuer
	_ = scheduler.EnsureURLExtraction(context.Background(), source.ID(), source.UpdatedAt())
	enqueuer.job.Attempts = 1
	return enqueuer.job
}

type urlExtractionRepositoryStub struct {
	source      *Source
	updateCount int
}

func (repository *urlExtractionRepositoryStub) GetSource(context.Context, string) (*Source, error) {
	return repository.source, nil
}

func (repository *urlExtractionRepositoryStub) UpdateSourceExtraction(context.Context, *Source) error {
	repository.updateCount++
	return nil
}

type urlExtractorStub struct {
	document ExtractedURLDocument
	err      error
}

func (extractor *urlExtractorStub) Extract(context.Context, string) (ExtractedURLDocument, error) {
	return extractor.document, extractor.err
}
