package sources

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHTTPHandlerCreatesListsAndLoadsManualSources(t *testing.T) {
	now := time.Date(2026, 7, 30, 15, 0, 0, 0, time.UTC)
	created, _ := NewManualSource("11111111-1111-4111-8111-111111111111", "PostgreSQL query planning", "Compare query plans", "EXPLAIN ANALYZE output", now)
	service := &sourceServiceStub{
		created: created,
		found:   created,
		page: SourcePage{
			Sources:    []SourceSummary{created.Summary()},
			Page:       1,
			PageSize:   50,
			TotalItems: 1,
		},
	}
	handler := NewHTTPHandler(service, allowSourceRequest)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	createRequest := httptest.NewRequest(http.MethodPost, "/api/v1/sources", strings.NewReader(`{"kind":"manual","title":"PostgreSQL query planning","captureNote":"Compare query plans","content":"EXPLAIN ANALYZE output"}`))
	createResponse := httptest.NewRecorder()
	mux.ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated || !strings.Contains(createResponse.Body.String(), `"processingStatus":"ready"`) || !strings.Contains(createResponse.Body.String(), "EXPLAIN ANALYZE output") {
		t.Fatalf("create response = %d %s", createResponse.Code, createResponse.Body.String())
	}
	if service.createInput.Title != "PostgreSQL query planning" || service.createInput.CaptureNote != "Compare query plans" {
		t.Errorf("create input = %+v", service.createInput)
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/sources?inbox=true&page=1&pageSize=50", nil)
	listResponse := httptest.NewRecorder()
	mux.ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK || !strings.Contains(listResponse.Body.String(), "PostgreSQL query planning") || strings.Contains(listResponse.Body.String(), "EXPLAIN ANALYZE output") {
		t.Fatalf("list response = %d %s", listResponse.Code, listResponse.Body.String())
	}
	if !service.listFilter.InboxOnly {
		t.Error("list did not request Source Inbox items")
	}

	spaceListRequest := httptest.NewRequest(http.MethodGet, "/api/v1/sources?spaceId=22222222-2222-4222-8222-222222222222", nil)
	spaceListResponse := httptest.NewRecorder()
	mux.ServeHTTP(spaceListResponse, spaceListRequest)
	if spaceListResponse.Code != http.StatusOK || service.listFilter.SpaceID != "22222222-2222-4222-8222-222222222222" {
		t.Fatalf("space list response = %d filter:%+v", spaceListResponse.Code, service.listFilter)
	}

	getRequest := httptest.NewRequest(http.MethodGet, "/api/v1/sources/11111111-1111-4111-8111-111111111111", nil)
	getResponse := httptest.NewRecorder()
	mux.ServeHTTP(getResponse, getRequest)
	if getResponse.Code != http.StatusOK || !strings.Contains(getResponse.Body.String(), "EXPLAIN ANALYZE output") {
		t.Fatalf("get response = %d %s", getResponse.Code, getResponse.Body.String())
	}

	organizeRequest := httptest.NewRequest(http.MethodPatch, "/api/v1/sources/11111111-1111-4111-8111-111111111111", strings.NewReader(`{"spaceId":"22222222-2222-4222-8222-222222222222"}`))
	organizeResponse := httptest.NewRecorder()
	mux.ServeHTTP(organizeResponse, organizeRequest)
	if organizeResponse.Code != http.StatusOK || service.organizeSpaceID != "22222222-2222-4222-8222-222222222222" {
		t.Fatalf("organize response = %d %s space:%q", organizeResponse.Code, organizeResponse.Body.String(), service.organizeSpaceID)
	}

	returnRequest := httptest.NewRequest(http.MethodPatch, "/api/v1/sources/11111111-1111-4111-8111-111111111111", strings.NewReader(`{"spaceId":null}`))
	returnResponse := httptest.NewRecorder()
	mux.ServeHTTP(returnResponse, returnRequest)
	if returnResponse.Code != http.StatusOK || service.organizeSpaceID != "" {
		t.Fatalf("return response = %d %s space:%q", returnResponse.Code, returnResponse.Body.String(), service.organizeSpaceID)
	}
}

func TestHTTPHandlerRejectsAmbiguousSourceLocationFilters(t *testing.T) {
	handler := NewHTTPHandler(&sourceServiceStub{}, allowSourceRequest)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/sources?inbox=true&spaceId=22222222-2222-4222-8222-222222222222", nil)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusUnprocessableEntity || !strings.Contains(response.Body.String(), "VALIDATION_ERROR") {
		t.Fatalf("response = %d %s", response.Code, response.Body.String())
	}
}

func TestHTTPHandlerRejectsUnsupportedSourceKinds(t *testing.T) {
	handler := NewHTTPHandler(&sourceServiceStub{}, allowSourceRequest)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/sources", strings.NewReader(`{"kind":"url","title":"Example"}`))
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusUnprocessableEntity || !strings.Contains(response.Body.String(), "SOURCE_KIND_NOT_SUPPORTED") {
		t.Fatalf("response = %d %s", response.Code, response.Body.String())
	}
}

func TestHTTPHandlerProtectsEverySourceRoute(t *testing.T) {
	protected := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
			http.Error(response, "owner required", http.StatusUnauthorized)
		})
	}
	handler := NewHTTPHandler(&sourceServiceStub{}, protected)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	for _, request := range []*http.Request{
		httptest.NewRequest(http.MethodGet, "/api/v1/sources", nil),
		httptest.NewRequest(http.MethodPost, "/api/v1/sources", strings.NewReader(`{"kind":"manual","title":"Example"}`)),
		httptest.NewRequest(http.MethodGet, "/api/v1/sources/11111111-1111-4111-8111-111111111111", nil),
		httptest.NewRequest(http.MethodPatch, "/api/v1/sources/11111111-1111-4111-8111-111111111111", strings.NewReader(`{"spaceId":null}`)),
	} {
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized {
			t.Errorf("%s %s response = %d", request.Method, request.URL.Path, response.Code)
		}
	}
}

func allowSourceRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		next.ServeHTTP(response, request)
	})
}

type sourceServiceStub struct {
	created         *Source
	found           *Source
	page            SourcePage
	createInput     CreateManualSourceInput
	listFilter      ListFilter
	organizeSpaceID string
}

func (service *sourceServiceStub) CreateManualSource(_ context.Context, input CreateManualSourceInput) (*Source, error) {
	service.createInput = input
	return service.created, nil
}

func (service *sourceServiceStub) GetSource(context.Context, string) (*Source, error) {
	return service.found, nil
}

func (service *sourceServiceStub) ListSources(_ context.Context, filter ListFilter) (SourcePage, error) {
	service.listFilter = filter
	return service.page, nil
}

func (service *sourceServiceStub) OrganizeSource(_ context.Context, _ string, spaceID string) (*Source, error) {
	service.organizeSpaceID = spaceID
	return service.found, nil
}
