package notes

import (
	"context"
	"database/sql"
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

func TestHTTPHandlerSeparatesOwnerAndAnonymousNoteRepresentations(t *testing.T) {
	note, _ := NewNote("33333333-3333-4333-8333-333333333333", "11111111-1111-4111-8111-111111111111", "", "Draft title", "unfinished secret draft")
	note.updatedAt = time.Date(2026, 7, 30, 12, 34, 0, 0, time.UTC)
	published := PublishedNote{
		ID:       note.ID(),
		SpaceID:  note.SpaceID(),
		Title:    "Published title",
		Slug:     "published-title",
		Markdown: "reviewed public content",
		PublishedAt: func() (value sql.NullTime) {
			value.Time = time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
			value.Valid = true
			return value
		}(),
	}
	repository := &noteRepositoryStub{
		notePage:          NotePage{Notes: []*Note{note}, Page: 1, PageSize: 20, TotalItems: 1},
		publishedNote:     published,
		publishedNotePage: PublishedNotePage{Notes: []PublishedNote{published}, Page: 1, PageSize: 20, TotalItems: 1},
	}
	service := NewService(ServiceConfig{Notes: repository, Knowledge: &knowledgeCatalogStub{}, GenerateID: idSequence("unused"), Now: time.Now})
	handler := NewHTTPHandler(service, allowNoteRequest)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	ownerRequest := httptest.NewRequest(http.MethodGet, "/api/v1/notes?spaceId=11111111-1111-4111-8111-111111111111&page=1&pageSize=20", nil)
	ownerResponse := httptest.NewRecorder()
	mux.ServeHTTP(ownerResponse, ownerRequest)
	if ownerResponse.Code != http.StatusOK || !strings.Contains(ownerResponse.Body.String(), "unfinished secret draft") || !strings.Contains(ownerResponse.Body.String(), `"updatedAt":"2026-07-30T12:34:00.000000000Z"`) {
		t.Fatalf("owner response = %d %s", ownerResponse.Code, ownerResponse.Body.String())
	}

	publicRequest := httptest.NewRequest(http.MethodGet, "/api/v1/public/notes/33333333-3333-4333-8333-333333333333", nil)
	publicResponse := httptest.NewRecorder()
	mux.ServeHTTP(publicResponse, publicRequest)
	if publicResponse.Code != http.StatusOK || !strings.Contains(publicResponse.Body.String(), "reviewed public content") || strings.Contains(publicResponse.Body.String(), "unfinished secret draft") {
		t.Fatalf("public response = %d %s", publicResponse.Code, publicResponse.Body.String())
	}

	publicListRequest := httptest.NewRequest(http.MethodGet, "/api/v1/public/spaces/11111111-1111-4111-8111-111111111111/notes?page=1&pageSize=20", nil)
	publicListResponse := httptest.NewRecorder()
	mux.ServeHTTP(publicListResponse, publicListRequest)
	if publicListResponse.Code != http.StatusOK || !strings.Contains(publicListResponse.Body.String(), "reviewed public content") {
		t.Fatalf("public list response = %d %s", publicListResponse.Code, publicListResponse.Body.String())
	}
}

func TestHTTPHandlerTrashRestoreAndPermanentDeleteCommands(t *testing.T) {
	space, _ := knowledge.NewSpace("11111111-1111-4111-8111-111111111111", "AI Agent", knowledge.VisibilityPublic)
	note, _ := NewNote("33333333-3333-4333-8333-333333333333", space.ID(), "", "Agent Loop", "reviewed")
	trashedAt := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	repository := &noteRepositoryStub{
		found:      note,
		trashEntry: TrashEntry{Note: note, TrashedAt: trashedAt},
		trashPage:  TrashPage{Entries: []TrashEntry{{Note: note, TrashedAt: trashedAt}}, Page: 1, PageSize: 20, TotalItems: 1},
	}
	service := NewService(ServiceConfig{
		Notes:      repository,
		Knowledge:  &knowledgeCatalogStub{space: space},
		GenerateID: idSequence("44444444-4444-4444-8444-444444444444"),
		Now:        func() time.Time { return trashedAt },
	})
	handler := NewHTTPHandler(service, allowNoteRequest)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	trashRequest := httptest.NewRequest(http.MethodPost, "/api/v1/notes/33333333-3333-4333-8333-333333333333/trash", strings.NewReader(`{"expectedVersion":1}`))
	trashResponse := httptest.NewRecorder()
	mux.ServeHTTP(trashResponse, trashRequest)
	if trashResponse.Code != http.StatusNoContent {
		t.Fatalf("trash response = %d %s", trashResponse.Code, trashResponse.Body.String())
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/trash/notes?page=1&pageSize=20", nil)
	listResponse := httptest.NewRecorder()
	mux.ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK || !strings.Contains(listResponse.Body.String(), note.ID()) {
		t.Fatalf("trash list response = %d %s", listResponse.Code, listResponse.Body.String())
	}

	restoreRequest := httptest.NewRequest(http.MethodPost, "/api/v1/trash/notes/33333333-3333-4333-8333-333333333333/restore", strings.NewReader(`{"confirmPublish":true}`))
	restoreResponse := httptest.NewRecorder()
	mux.ServeHTTP(restoreResponse, restoreRequest)
	if restoreResponse.Code != http.StatusOK || !strings.Contains(restoreResponse.Body.String(), note.ID()) {
		t.Fatalf("restore response = %d %s", restoreResponse.Code, restoreResponse.Body.String())
	}

	deleteRequest := httptest.NewRequest(http.MethodDelete, "/api/v1/trash/notes/33333333-3333-4333-8333-333333333333", nil)
	deleteResponse := httptest.NewRecorder()
	mux.ServeHTTP(deleteResponse, deleteRequest)
	if deleteResponse.Code != http.StatusNoContent {
		t.Fatalf("delete response = %d %s", deleteResponse.Code, deleteResponse.Body.String())
	}
}

func TestHTTPHandlerMovesPrivateNoteToPublicWithConfirmation(t *testing.T) {
	privateSpace, _ := knowledge.NewSpace("11111111-1111-4111-8111-111111111111", "Private", knowledge.VisibilityPrivate)
	publicSpace, _ := knowledge.NewSpace("22222222-2222-4222-8222-222222222222", "Public", knowledge.VisibilityPublic)
	note, _ := NewNote("33333333-3333-4333-8333-333333333333", privateSpace.ID(), "", "Memory", "reviewed")
	repository := &noteRepositoryStub{found: note}
	service := NewService(ServiceConfig{
		Notes: repository,
		Knowledge: &knowledgeCatalogStub{spaces: map[string]*knowledge.Space{
			privateSpace.ID(): privateSpace,
			publicSpace.ID():  publicSpace,
		}},
		GenerateID: idSequence("44444444-4444-4444-8444-444444444444"),
		Now:        time.Now,
	})
	handler := NewHTTPHandler(service, allowNoteRequest)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/notes/33333333-3333-4333-8333-333333333333/move", strings.NewReader(`{"expectedVersion":1,"spaceId":"22222222-2222-4222-8222-222222222222","directoryId":null,"confirmPublish":true}`))
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"spaceId":"22222222-2222-4222-8222-222222222222"`) || !strings.Contains(response.Body.String(), `"published"`) {
		t.Fatalf("move response = %d %s", response.Code, response.Body.String())
	}
}

func allowNoteRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		next.ServeHTTP(response, request.WithContext(context.Background()))
	})
}
