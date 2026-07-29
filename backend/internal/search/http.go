package search

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Ken-Chy129/note-foundry/backend/internal/platform/httpapi"
)

type OwnerMiddleware func(http.Handler) http.Handler

type HTTPHandler struct {
	service      *Service
	requireOwner OwnerMiddleware
}

func NewHTTPHandler(service *Service, requireOwner OwnerMiddleware) *HTTPHandler {
	return &HTTPHandler{service: service, requireOwner: requireOwner}
}

func (handler *HTTPHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/search", handler.requireOwner(http.HandlerFunc(handler.searchOwner)))
	mux.HandleFunc("GET /api/v1/public/search", handler.searchPublic)
}

type resultResponse struct {
	ID      string  `json:"id"`
	SpaceID string  `json:"spaceId"`
	Title   string  `json:"title"`
	Slug    string  `json:"slug"`
	Snippet string  `json:"snippet"`
	Rank    float64 `json:"rank"`
}

func (handler *HTTPHandler) searchOwner(response http.ResponseWriter, request *http.Request) {
	handler.runSearch(response, request, false)
}

func (handler *HTTPHandler) searchPublic(response http.ResponseWriter, request *http.Request) {
	handler.runSearch(response, request, true)
}

func (handler *HTTPHandler) runSearch(response http.ResponseWriter, request *http.Request, public bool) {
	page, err := searchPaginationValue(request, "page", 1, 1, 1_000_000)
	if err != nil {
		httpapi.WriteError(response, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "page must be a positive integer")
		return
	}
	pageSize, err := searchPaginationValue(request, "pageSize", 20, 1, 100)
	if err != nil {
		httpapi.WriteError(response, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "pageSize must be between 1 and 100")
		return
	}
	options := Options{
		Query:    request.URL.Query().Get("q"),
		SpaceID:  request.URL.Query().Get("spaceId"),
		TagID:    request.URL.Query().Get("tagId"),
		Page:     page,
		PageSize: pageSize,
	}
	var result Page
	if public {
		result, err = handler.service.SearchPublic(request.Context(), options)
	} else {
		result, err = handler.service.SearchOwner(request.Context(), options)
	}
	if errors.Is(err, ErrQueryRequired) {
		httpapi.WriteError(response, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error())
		return
	}
	if err != nil {
		httpapi.WriteError(response, http.StatusInternalServerError, "INTERNAL_ERROR", "could not search Learning Notes")
		return
	}
	data := make([]resultResponse, 0, len(result.Results))
	for _, item := range result.Results {
		data = append(data, resultResponse{ID: item.ID, SpaceID: item.SpaceID, Title: item.Title, Slug: item.Slug, Snippet: item.Snippet, Rank: item.Rank})
	}
	totalPages := 0
	if result.TotalItems > 0 {
		totalPages = (result.TotalItems + result.PageSize - 1) / result.PageSize
	}
	_ = httpapi.WriteJSON(response, http.StatusOK, map[string]any{
		"data": data,
		"pagination": map[string]int{
			"page":       result.Page,
			"pageSize":   result.PageSize,
			"totalItems": result.TotalItems,
			"totalPages": totalPages,
		},
	})
}

func searchPaginationValue(request *http.Request, key string, fallback, minimum, maximum int) (int, error) {
	raw := request.URL.Query().Get(key)
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < minimum || value > maximum {
		return 0, errors.New("invalid pagination value")
	}
	return value, nil
}
