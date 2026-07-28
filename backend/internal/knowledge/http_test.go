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

func allowRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		next.ServeHTTP(response, request.WithContext(context.Background()))
	})
}
