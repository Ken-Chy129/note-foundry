package search

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHTTPHandlerServesOwnerAndPublicPaginatedSearch(t *testing.T) {
	repository := &repositoryStub{page: Page{
		Results:    []Result{{ID: "11111111-1111-4111-8111-111111111111", SpaceID: "22222222-2222-4222-8222-222222222222", Title: "Memory", Slug: "memory", Snippet: "memory body", Rank: 1.5, UpdatedAt: time.Date(2026, 7, 30, 12, 34, 0, 0, time.UTC)}},
		Page:       1,
		PageSize:   20,
		TotalItems: 1,
	}}
	handler := NewHTTPHandler(NewService(repository), allowSearchRequest)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	for _, path := range []string{"/api/v1/search?q=memory&page=1&pageSize=20", "/api/v1/public/search?q=memory&page=1&pageSize=20"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "memory body") || !strings.Contains(response.Body.String(), `"updatedAt":"2026-07-30T12:34:00.000000000Z"`) {
			t.Errorf("%s response = %d %s", path, response.Code, response.Body.String())
		}
	}
}

func TestHTTPHandlerRejectsEmptySearchQuery(t *testing.T) {
	handler := NewHTTPHandler(NewService(&repositoryStub{}), allowSearchRequest)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=", nil)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("response = %d %s", response.Code, response.Body.String())
	}
}

func allowSearchRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		next.ServeHTTP(response, request.WithContext(context.Background()))
	})
}
