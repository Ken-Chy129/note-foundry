package sources

import (
	"context"
	"fmt"
	"time"

	"github.com/Ken-Chy129/note-foundry/backend/internal/jobs"
)

type ExtractedURLDocument struct {
	Title   string
	Content string
}

type URLDocumentExtractor interface {
	Extract(context.Context, string) (ExtractedURLDocument, error)
}

type URLExtractionRepository interface {
	GetSource(context.Context, string) (*Source, error)
	UpdateSourceExtraction(context.Context, *Source) error
}

type URLExtractionHandler struct {
	repository URLExtractionRepository
	extractor  URLDocumentExtractor
	now        func() time.Time
}

func NewURLExtractionHandler(repository URLExtractionRepository, extractor URLDocumentExtractor, now func() time.Time) *URLExtractionHandler {
	return &URLExtractionHandler{repository: repository, extractor: extractor, now: now}
}

func (handler *URLExtractionHandler) Handle(ctx context.Context, job jobs.Job) error {
	payload, err := DecodeURLExtractionPayload(job)
	if err != nil {
		return err
	}
	source, err := handler.repository.GetSource(ctx, payload.SourceID)
	if err != nil {
		return fmt.Errorf("load URL Learning Source: %w", err)
	}
	if err := source.MarkExtractionProcessing(handler.now()); err != nil {
		return err
	}
	if err := handler.repository.UpdateSourceExtraction(ctx, source); err != nil {
		return fmt.Errorf("mark URL extraction processing: %w", err)
	}
	document, err := handler.extractor.Extract(ctx, source.OriginalURL())
	if err != nil {
		if job.Attempts < job.MaxAttempts {
			_ = source.RetryExtraction(err.Error(), handler.now())
		} else {
			_ = source.FailExtraction(err.Error(), handler.now())
		}
		if updateErr := handler.repository.UpdateSourceExtraction(ctx, source); updateErr != nil {
			return fmt.Errorf("extract URL: %w; record failure: %v", err, updateErr)
		}
		return fmt.Errorf("extract URL: %w", err)
	}
	extractedTitle := document.Title
	if source.Title() != defaultURLSourceTitle(source.NormalizedURL()) {
		extractedTitle = ""
	}
	if err := source.CompleteExtraction(extractedTitle, document.Content, handler.now()); err != nil {
		return err
	}
	if err := handler.repository.UpdateSourceExtraction(ctx, source); err != nil {
		return fmt.Errorf("store URL extraction: %w", err)
	}
	return nil
}
