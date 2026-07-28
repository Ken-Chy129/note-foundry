package knowledge

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPHandlerCreatesOwnerKnowledgeSpace(t *testing.T) {
	repository := &spaceRepositoryStub{}
	service := NewService(ServiceConfig{
		Spaces:     repository,
		GenerateID: func() string { return "11111111-1111-4111-8111-111111111111" },
	})
	handler := NewHTTPHandler(service, allowRequest)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/spaces", strings.NewReader(`{"name":"AI Agent","visibility":"public"}`))
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status code = %d, want %d; body = %s", response.Code, http.StatusCreated, response.Body.String())
	}
	if got := response.Header().Get("Location"); got != "/api/v1/spaces/11111111-1111-4111-8111-111111111111" {
		t.Errorf("Location = %q", got)
	}
	var body spaceResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.ID != "11111111-1111-4111-8111-111111111111" || body.Name != "AI Agent" || body.Visibility != VisibilityPublic {
		t.Errorf("space response = %+v", body)
	}
}

func TestHTTPHandlerRejectsInvalidKnowledgeSpaceInput(t *testing.T) {
	service := NewService(ServiceConfig{
		Spaces:     &spaceRepositoryStub{},
		GenerateID: func() string { return "11111111-1111-4111-8111-111111111111" },
	})
	handler := NewHTTPHandler(service, allowRequest)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/spaces", strings.NewReader(`{"name":"AI Agent","visibility":"unlisted"}`))
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status code = %d, want %d", response.Code, http.StatusUnprocessableEntity)
	}
}

func TestHTTPHandlerListsPaginatedKnowledgeSpaces(t *testing.T) {
	space, _ := NewSpace("11111111-1111-4111-8111-111111111111", "AI Agent", VisibilityPublic)
	service := NewService(ServiceConfig{
		Spaces:     &spaceRepositoryStub{spaces: []*Space{space}},
		GenerateID: func() string { return "unused" },
	})
	handler := NewHTTPHandler(service, allowRequest)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/spaces?page=1&pageSize=20", nil)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", response.Code, http.StatusOK)
	}
	var body struct {
		Data       []spaceResponse `json:"data"`
		Pagination struct {
			Page       int `json:"page"`
			PageSize   int `json:"pageSize"`
			TotalItems int `json:"totalItems"`
			TotalPages int `json:"totalPages"`
		} `json:"pagination"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Data) != 1 || body.Pagination.Page != 1 || body.Pagination.PageSize != 20 || body.Pagination.TotalItems != 1 || body.Pagination.TotalPages != 1 {
		t.Errorf("list response = %+v", body)
	}
}

func TestHTTPHandlerRenamesKnowledgeSpace(t *testing.T) {
	space, _ := NewSpace("11111111-1111-4111-8111-111111111111", "AI", VisibilityPrivate)
	service := NewService(ServiceConfig{
		Spaces:     &spaceRepositoryStub{found: space},
		GenerateID: func() string { return "unused" },
	})
	handler := NewHTTPHandler(service, allowRequest)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	request := httptest.NewRequest(http.MethodPatch, "/api/v1/spaces/11111111-1111-4111-8111-111111111111", strings.NewReader(`{"name":"AI Agent"}`))
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d; body = %s", response.Code, http.StatusOK, response.Body.String())
	}
	if space.Name() != "AI Agent" {
		t.Errorf("space name = %q", space.Name())
	}
}

func TestHTTPHandlerCreatesNestedDirectory(t *testing.T) {
	space, _ := NewSpace("11111111-1111-4111-8111-111111111111", "AI Agent", VisibilityPublic)
	parent, _ := NewDirectory("22222222-2222-4222-8222-222222222222", space.ID(), "", "Agents")
	directories := &directoryRepositoryStub{found: parent}
	service := NewService(ServiceConfig{
		Spaces:      &spaceRepositoryStub{found: space},
		Directories: directories,
		GenerateID:  func() string { return "33333333-3333-4333-8333-333333333333" },
	})
	handler := NewHTTPHandler(service, allowRequest)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/spaces/11111111-1111-4111-8111-111111111111/directories", strings.NewReader(`{"name":"Hermes Agent","parentId":"22222222-2222-4222-8222-222222222222"}`))
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status code = %d, want %d; body = %s", response.Code, http.StatusCreated, response.Body.String())
	}
	if directories.created == nil || directories.created.ParentID() != parent.ID() {
		t.Errorf("created directory = %+v", directories.created)
	}
}

func TestHTTPHandlerListsDirectoryHierarchy(t *testing.T) {
	space, _ := NewSpace("11111111-1111-4111-8111-111111111111", "AI Agent", VisibilityPublic)
	root, _ := NewDirectory("22222222-2222-4222-8222-222222222222", space.ID(), "", "Hermes Agent")
	child, _ := NewDirectory("33333333-3333-4333-8333-333333333333", space.ID(), root.ID(), "Architecture")
	service := NewService(ServiceConfig{
		Spaces:      &spaceRepositoryStub{found: space},
		Directories: &directoryRepositoryStub{directories: []*Directory{root, child}},
		GenerateID:  func() string { return "unused" },
	})
	handler := NewHTTPHandler(service, allowRequest)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/spaces/11111111-1111-4111-8111-111111111111/directories", nil)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", response.Code, http.StatusOK)
	}
	var body struct {
		Data []directoryResponse `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Data) != 2 || body.Data[0].ParentID != nil || body.Data[1].ParentID == nil || *body.Data[1].ParentID != root.ID() {
		t.Errorf("directory response = %+v", body.Data)
	}
}

func TestHTTPHandlerRejectsCyclicDirectoryMove(t *testing.T) {
	space, _ := NewSpace("11111111-1111-4111-8111-111111111111", "AI Agent", VisibilityPublic)
	directory, _ := NewDirectory("22222222-2222-4222-8222-222222222222", space.ID(), "", "Hermes Agent")
	child, _ := NewDirectory("33333333-3333-4333-8333-333333333333", space.ID(), directory.ID(), "Architecture")
	service := NewService(ServiceConfig{
		Spaces: &spaceRepositoryStub{found: space},
		Directories: &directoryRepositoryStub{
			foundByID:  map[string]*Directory{directory.ID(): directory, child.ID(): child},
			wouldCycle: true,
		},
		GenerateID: func() string { return "unused" },
	})
	handler := NewHTTPHandler(service, allowRequest)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/directories/22222222-2222-4222-8222-222222222222/move", strings.NewReader(`{"parentId":"33333333-3333-4333-8333-333333333333"}`))
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusConflict {
		t.Fatalf("status code = %d, want %d", response.Code, http.StatusConflict)
	}
}

func TestHTTPHandlerCreatesAndListsGlobalTags(t *testing.T) {
	tags := &tagRepositoryStub{}
	service := NewService(ServiceConfig{
		Spaces:      &spaceRepositoryStub{},
		Directories: &directoryRepositoryStub{},
		Tags:        tags,
		GenerateID:  func() string { return "11111111-1111-4111-8111-111111111111" },
	})
	handler := NewHTTPHandler(service, allowRequest)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	createRequest := httptest.NewRequest(http.MethodPost, "/api/v1/tags", strings.NewReader(`{"name":"memory"}`))
	createResponse := httptest.NewRecorder()
	mux.ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d", createResponse.Code, http.StatusCreated)
	}
	if tags.created == nil {
		t.Fatal("tag was not created")
	}
	tags.tags = []*Tag{tags.created}

	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/tags?page=1&pageSize=20", nil)
	listResponse := httptest.NewRecorder()
	mux.ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d", listResponse.Code, http.StatusOK)
	}
	var body struct {
		Data []tagResponse `json:"data"`
	}
	if err := json.NewDecoder(listResponse.Body).Decode(&body); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(body.Data) != 1 || body.Data[0].Name != "memory" {
		t.Errorf("tag list = %+v", body.Data)
	}
}

func TestHTTPHandlerListsPublicKnowledgeNavigationWithoutOwnerSession(t *testing.T) {
	space, _ := NewSpace("11111111-1111-4111-8111-111111111111", "AI Agent", VisibilityPublic)
	directory, _ := NewDirectory("22222222-2222-4222-8222-222222222222", space.ID(), "", "Hermes Agent")
	service := NewService(ServiceConfig{
		Spaces:      &spaceRepositoryStub{spaces: []*Space{space}, found: space},
		Directories: &directoryRepositoryStub{directories: []*Directory{directory}},
		Tags:        &tagRepositoryStub{},
		GenerateID:  func() string { return "unused" },
	})
	handler := NewHTTPHandler(service, func(http.Handler) http.Handler {
		return http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
			response.WriteHeader(http.StatusTeapot)
		})
	})
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	spacesRequest := httptest.NewRequest(http.MethodGet, "/api/v1/public/spaces?page=1&pageSize=20", nil)
	spacesResponse := httptest.NewRecorder()
	mux.ServeHTTP(spacesResponse, spacesRequest)
	if spacesResponse.Code != http.StatusOK || !strings.Contains(spacesResponse.Body.String(), "AI Agent") {
		t.Fatalf("public spaces response = %d %s", spacesResponse.Code, spacesResponse.Body.String())
	}

	directoriesRequest := httptest.NewRequest(http.MethodGet, "/api/v1/public/spaces/11111111-1111-4111-8111-111111111111/directories", nil)
	directoriesResponse := httptest.NewRecorder()
	mux.ServeHTTP(directoriesResponse, directoriesRequest)
	if directoriesResponse.Code != http.StatusOK || !strings.Contains(directoriesResponse.Body.String(), "Hermes Agent") {
		t.Fatalf("public directories response = %d %s", directoriesResponse.Code, directoriesResponse.Body.String())
	}
}

func TestHTTPHandlerSetsLearningNoteTagsAndMergesGlobalTags(t *testing.T) {
	tag, _ := NewTag("11111111-1111-4111-8111-111111111111", "memory")
	tags := &tagRepositoryStub{tags: []*Tag{tag}, merged: tag}
	service := NewService(ServiceConfig{Tags: tags, GenerateID: func() string { return "unused" }})
	handler := NewHTTPHandler(service, allowRequest)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	setRequest := httptest.NewRequest(http.MethodPut, "/api/v1/notes/22222222-2222-4222-8222-222222222222/tags", strings.NewReader(`{"tagIds":["11111111-1111-4111-8111-111111111111"]}`))
	setResponse := httptest.NewRecorder()
	mux.ServeHTTP(setResponse, setRequest)
	if setResponse.Code != http.StatusOK || !strings.Contains(setResponse.Body.String(), "memory") {
		t.Fatalf("set Note Tags response = %d %s", setResponse.Code, setResponse.Body.String())
	}

	mergeRequest := httptest.NewRequest(http.MethodPost, "/api/v1/tags/33333333-3333-4333-8333-333333333333/merge", strings.NewReader(`{"targetTagId":"11111111-1111-4111-8111-111111111111"}`))
	mergeResponse := httptest.NewRecorder()
	mux.ServeHTTP(mergeResponse, mergeRequest)
	if mergeResponse.Code != http.StatusOK || !strings.Contains(mergeResponse.Body.String(), "memory") {
		t.Fatalf("merge Tag response = %d %s", mergeResponse.Code, mergeResponse.Body.String())
	}
}

func TestHTTPHandlerReturnsDerivedForwardLinksAndBacklinks(t *testing.T) {
	link := LinkedNote{ID: "11111111-1111-4111-8111-111111111111", SpaceID: "22222222-2222-4222-8222-222222222222", Title: "Memory", Slug: "memory"}
	service := NewService(ServiceConfig{Links: &noteLinkRepositoryStub{links: []LinkedNote{link}}})
	handler := NewHTTPHandler(service, allowRequest)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	for _, path := range []string{
		"/api/v1/notes/33333333-3333-4333-8333-333333333333/links",
		"/api/v1/notes/33333333-3333-4333-8333-333333333333/backlinks",
		"/api/v1/public/notes/33333333-3333-4333-8333-333333333333/links",
		"/api/v1/public/notes/33333333-3333-4333-8333-333333333333/backlinks",
	} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Memory") {
			t.Errorf("%s response = %d %s", path, response.Code, response.Body.String())
		}
	}
}

func allowRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		next.ServeHTTP(response, request.WithContext(context.Background()))
	})
}
