package notes

import (
	"context"
	"errors"
	"fmt"
)

var (
	ErrBulkPublishFailed       = errors.New("bulk publication failed")
	ErrInvalidBulkPublishLimit = errors.New("bulk publication limit must be positive")
)

// PublicDraftCandidate identifies an unpublished Learning Note that already
// belongs to a public Knowledge Space. The title and space name are included so
// operational tools can present an auditable preflight before publishing.
type PublicDraftCandidate struct {
	ID        string
	Title     string
	SpaceName string
	Version   int64
}

type PublicDraftCatalog interface {
	ListUnpublishedPublicNotes(context.Context, int) ([]PublicDraftCandidate, error)
}

type NotePublisher interface {
	Publish(context.Context, string, int64) (*Note, error)
}

type BulkPublishResult struct {
	Candidates int
	Published  int
}

// PublishPublicDrafts deliberately delegates every candidate to Service.Publish
// instead of mutating published columns directly. This preserves revisions and
// all rebuildable public projections such as links, search, and attachments.
func PublishPublicDrafts(ctx context.Context, catalog PublicDraftCatalog, publisher NotePublisher, limit int) (BulkPublishResult, error) {
	if limit <= 0 {
		return BulkPublishResult{}, ErrInvalidBulkPublishLimit
	}
	candidates, err := catalog.ListUnpublishedPublicNotes(ctx, limit)
	if err != nil {
		return BulkPublishResult{}, fmt.Errorf("list unpublished Learning Notes in public Knowledge Spaces: %w", err)
	}
	result := BulkPublishResult{Candidates: len(candidates)}
	for _, candidate := range candidates {
		if _, err := publisher.Publish(ctx, candidate.ID, candidate.Version); err != nil {
			return result, fmt.Errorf("%w: publish %q in %q: %v", ErrBulkPublishFailed, candidate.Title, candidate.SpaceName, err)
		}
		result.Published++
	}
	return result, nil
}
