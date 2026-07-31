package notes

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type publicDraftCatalogStub struct {
	candidates []PublicDraftCandidate
	err        error
}

func (stub publicDraftCatalogStub) ListUnpublishedPublicNotes(context.Context, int) ([]PublicDraftCandidate, error) {
	return stub.candidates, stub.err
}

type notePublisherStub struct {
	published []PublicDraftCandidate
	errForID  string
}

func (stub *notePublisherStub) Publish(_ context.Context, id string, expectedVersion int64) (*Note, error) {
	if id == stub.errForID {
		return nil, errors.New("publish failed")
	}
	stub.published = append(stub.published, PublicDraftCandidate{ID: id, Version: expectedVersion})
	return &Note{}, nil
}

func TestPublishPublicDraftsUsesTheNormalPublishFlowAtEachCurrentVersion(t *testing.T) {
	candidates := []PublicDraftCandidate{
		{ID: "note-1", Title: "Java 集合", SpaceName: "Java 与 JVM", Version: 3},
		{ID: "note-2", Title: "垃圾回收", SpaceName: "Java 与 JVM", Version: 7},
	}
	publisher := &notePublisherStub{}

	result, err := PublishPublicDrafts(context.Background(), publicDraftCatalogStub{candidates: candidates}, publisher, 100)
	if err != nil {
		t.Fatalf("PublishPublicDrafts() error = %v", err)
	}
	if result.Published != len(candidates) {
		t.Fatalf("Published = %d, want %d", result.Published, len(candidates))
	}
	want := []PublicDraftCandidate{{ID: "note-1", Version: 3}, {ID: "note-2", Version: 7}}
	if !reflect.DeepEqual(publisher.published, want) {
		t.Fatalf("published = %#v, want %#v", publisher.published, want)
	}
}

func TestPublishPublicDraftsStopsAndNamesTheFailedArticle(t *testing.T) {
	candidates := []PublicDraftCandidate{
		{ID: "note-1", Title: "Java 集合", SpaceName: "Java 与 JVM", Version: 1},
		{ID: "note-2", Title: "垃圾回收", SpaceName: "Java 与 JVM", Version: 2},
	}
	publisher := &notePublisherStub{errForID: "note-2"}

	result, err := PublishPublicDrafts(context.Background(), publicDraftCatalogStub{candidates: candidates}, publisher, 100)
	if err == nil || !errors.Is(err, ErrBulkPublishFailed) {
		t.Fatalf("PublishPublicDrafts() error = %v, want ErrBulkPublishFailed", err)
	}
	if result.Published != 1 {
		t.Fatalf("Published = %d, want 1", result.Published)
	}
	if got := err.Error(); got == "" || !containsAll(got, "Java 与 JVM", "垃圾回收") {
		t.Fatalf("error = %q, want space and title", got)
	}
}

func containsAll(value string, parts ...string) bool {
	for _, part := range parts {
		if len(part) > 0 && !contains(value, part) {
			return false
		}
	}
	return true
}

func contains(value, part string) bool {
	for index := 0; index+len(part) <= len(value); index++ {
		if value[index:index+len(part)] == part {
			return true
		}
	}
	return false
}
