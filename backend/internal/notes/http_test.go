package notes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Ken-Chy129/note-foundry/backend/internal/knowledge"
)

func TestHTTPHandlerCreatesAutosavesAndPublishesLearningNote(t *testing.T) {
	space, _ := knowledge.NewSpace("11111111-1111-4111-8111-111111111111", "AI Agent", knowledge.VisibilityPublic)
	repository := &noteRepositoryStub{}
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	service := NewService(ServiceConfig{
		Notes:      repository,
		Knowledge:  &knowledgeCatalogStub{space: space},
		GenerateID: idSequence("33333333-3333-4333-8333-333333333333", "44444444-4444-4444-8444-444444444444"),
		Now:        func() time.Time { return now },
	})
	handler := NewHTTPHandler(service, allowNoteRequest)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	createRequest := httptest.NewRequest(http.MethodPost, "/api/v1/notes", strings.NewReader(`{"spaceId":"11111111-1111-4111-8111-111111111111","title":"Agent Loop","markdown":"draft"}`))
	createResponse := httptest.NewRecorder()
	mux.ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", createResponse.Code, createResponse.Body.String())
	}
	repository.found = repository.created

	saveRequest := httptest.NewRequest(http.MethodPatch, "/api/v1/notes/33333333-3333-4333-8333-333333333333", strings.NewReader(`{"expectedVersion":1,"title":"Agent Loop","markdown":"complete"}`))
	saveResponse := httptest.NewRecorder()
	mux.ServeHTTP(saveResponse, saveRequest)
	if saveResponse.Code != http.StatusOK {
		t.Fatalf("save status = %d, body = %s", saveResponse.Code, saveResponse.Body.String())
	}

	publishRequest := httptest.NewRequest(http.MethodPost, "/api/v1/notes/33333333-3333-4333-8333-333333333333/publish", strings.NewReader(`{"expectedVersion":2}`))
	publishResponse := httptest.NewRecorder()
	mux.ServeHTTP(publishResponse, publishRequest)
	if publishResponse.Code != http.StatusOK {
		t.Fatalf("publish status = %d, body = %s", publishResponse.Code, publishResponse.Body.String())
	}
	var body noteResponse
	if err := json.NewDecoder(publishResponse.Body).Decode(&body); err != nil {
		t.Fatalf("decode publish response: %v", err)
	}
	if body.Version != 2 || body.Published == nil || body.Published.Markdown != "complete" {
		t.Errorf("published response = %+v", body)
	}
}

func TestHTTPHandlerReturnsConflictForStaleAutosave(t *testing.T) {
	space, _ := knowledge.NewSpace("11111111-1111-4111-8111-111111111111", "AI Agent", knowledge.VisibilityPublic)
	note, _ := NewNote("33333333-3333-4333-8333-333333333333", space.ID(), "", "Agent Loop", "draft")
	service := NewService(ServiceConfig{
		Notes:      &noteRepositoryStub{found: note},
		Knowledge:  &knowledgeCatalogStub{space: space},
		GenerateID: idSequence("unused"),
		Now:        time.Now,
	})
	handler := NewHTTPHandler(service, allowNoteRequest)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	request := httptest.NewRequest(http.MethodPatch, "/api/v1/notes/33333333-3333-4333-8333-333333333333", strings.NewReader(`{"expectedVersion":9,"title":"Agent Loop","markdown":"stale"}`))
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "NOTE_VERSION_CONFLICT") {
		t.Fatalf("response = %d %s", response.Code, response.Body.String())
	}
}

func TestHTTPHandlerCreatesListsAndRestoresNoteRevision(t *testing.T) {
	space, _ := knowledge.NewSpace("11111111-1111-4111-8111-111111111111", "AI Agent", knowledge.VisibilityPublic)
	note, _ := NewNote("33333333-3333-4333-8333-333333333333", space.ID(), "", "Current", "current draft")
	target := Revision{
		ID:        "44444444-4444-4444-8444-444444444444",
		NoteID:    note.ID(),
		Title:     "Earlier",
		Slug:      "earlier",
		Markdown:  "earlier draft",
		Reason:    RevisionReasonPublish,
		CreatedAt: time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC),
	}
	repository := &noteRepositoryStub{
		found:        note,
		revision:     target,
		revisionPage: RevisionPage{Revisions: []Revision{target}, Page: 1, PageSize: 20, TotalItems: 1},
	}
	service := NewService(ServiceConfig{
		Notes:      repository,
		Knowledge:  &knowledgeCatalogStub{space: space},
		GenerateID: idSequence("55555555-5555-4555-8555-555555555555", "66666666-6666-4666-8666-666666666666"),
		Now:        func() time.Time { return time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC) },
	})
	handler := NewHTTPHandler(service, allowNoteRequest)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	checkpointRequest := httptest.NewRequest(http.MethodPost, "/api/v1/notes/33333333-3333-4333-8333-333333333333/revisions", strings.NewReader(`{"expectedVersion":1}`))
	checkpointResponse := httptest.NewRecorder()
	mux.ServeHTTP(checkpointResponse, checkpointRequest)
	if checkpointResponse.Code != http.StatusCreated {
		t.Fatalf("checkpoint response = %d %s", checkpointResponse.Code, checkpointResponse.Body.String())
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/notes/33333333-3333-4333-8333-333333333333/revisions?page=1&pageSize=20", nil)
	listResponse := httptest.NewRecorder()
	mux.ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK || !strings.Contains(listResponse.Body.String(), target.ID) {
		t.Fatalf("list response = %d %s", listResponse.Code, listResponse.Body.String())
	}

	restoreRequest := httptest.NewRequest(http.MethodPost, "/api/v1/notes/33333333-3333-4333-8333-333333333333/revisions/44444444-4444-4444-8444-444444444444/restore", strings.NewReader(`{"expectedVersion":1}`))
	restoreResponse := httptest.NewRecorder()
	mux.ServeHTTP(restoreResponse, restoreRequest)
	if restoreResponse.Code != http.StatusOK || !strings.Contains(restoreResponse.Body.String(), "earlier draft") {
		t.Fatalf("restore response = %d %s", restoreResponse.Code, restoreResponse.Body.String())
	}
}

func allowNoteRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		next.ServeHTTP(response, request.WithContext(context.Background()))
	})
}
