package sources

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/Ken-Chy129/note-foundry/backend/internal/platform/httpapi"
)

const maxSourceRequestBytes = 2 * 1024 * 1024

type OwnerMiddleware func(http.Handler) http.Handler

type sourceService interface {
	CreateManualSource(context.Context, CreateManualSourceInput) (*Source, error)
	GetSource(context.Context, string) (*Source, error)
	ListSources(context.Context, ListFilter) (SourcePage, error)
}

type HTTPHandler struct {
	service      sourceService
	requireOwner OwnerMiddleware
}

type sourceResponse struct {
	ID               string           `json:"id"`
	Kind             Kind             `json:"kind"`
	SpaceID          *string          `json:"spaceId"`
	Title            string           `json:"title"`
	CaptureNote      string           `json:"captureNote"`
	Content          string           `json:"content"`
	ProcessingStatus ProcessingStatus `json:"processingStatus"`
	CreatedAt        string           `json:"createdAt"`
	UpdatedAt        string           `json:"updatedAt"`
}

type sourceSummaryResponse struct {
	ID               string           `json:"id"`
	Kind             Kind             `json:"kind"`
	SpaceID          *string          `json:"spaceId"`
	Title            string           `json:"title"`
	CaptureNote      string           `json:"captureNote"`
	ProcessingStatus ProcessingStatus `json:"processingStatus"`
	CreatedAt        string           `json:"createdAt"`
	UpdatedAt        string           `json:"updatedAt"`
}

func NewHTTPHandler(service sourceService, requireOwner OwnerMiddleware) *HTTPHandler {
	return &HTTPHandler{service: service, requireOwner: requireOwner}
}

func (handler *HTTPHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/sources", handler.requireOwner(http.HandlerFunc(handler.listSources)))
	mux.Handle("POST /api/v1/sources", handler.requireOwner(http.HandlerFunc(handler.createSource)))
	mux.Handle("GET /api/v1/sources/{sourceId}", handler.requireOwner(http.HandlerFunc(handler.getSource)))
}

func (handler *HTTPHandler) createSource(response http.ResponseWriter, request *http.Request) {
	var input struct {
		Kind        Kind   `json:"kind"`
		Title       string `json:"title"`
		CaptureNote string `json:"captureNote"`
		Content     string `json:"content"`
	}
	if err := httpapi.DecodeJSON(request.Body, maxSourceRequestBytes, &input); err != nil {
		writeSourceDecodeError(response, err)
		return
	}
	if input.Kind != KindManual {
		httpapi.WriteError(response, http.StatusUnprocessableEntity, "SOURCE_KIND_NOT_SUPPORTED", "only manual Learning Sources are supported in this release slice")
		return
	}
	source, err := handler.service.CreateManualSource(request.Context(), CreateManualSourceInput{
		Title:       input.Title,
		CaptureNote: input.CaptureNote,
		Content:     input.Content,
	})
	if writeSourceServiceError(response, err, "create") {
		return
	}
	response.Header().Set("Location", "/api/v1/sources/"+source.ID())
	_ = httpapi.WriteJSON(response, http.StatusCreated, toSourceResponse(source))
}

func (handler *HTTPHandler) getSource(response http.ResponseWriter, request *http.Request) {
	source, err := handler.service.GetSource(request.Context(), request.PathValue("sourceId"))
	if writeSourceServiceError(response, err, "load") {
		return
	}
	_ = httpapi.WriteJSON(response, http.StatusOK, toSourceResponse(source))
}

func (handler *HTTPHandler) listSources(response http.ResponseWriter, request *http.Request) {
	page, pageSize, ok := sourcePagination(response, request)
	if !ok {
		return
	}
	inboxOnly := false
	if raw := request.URL.Query().Get("inbox"); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			httpapi.WriteError(response, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "inbox must be true or false")
			return
		}
		inboxOnly = value
	}
	pageResult, err := handler.service.ListSources(request.Context(), ListFilter{InboxOnly: inboxOnly, Page: page, PageSize: pageSize})
	if writeSourceServiceError(response, err, "list") {
		return
	}
	data := make([]sourceSummaryResponse, 0, len(pageResult.Sources))
	for _, source := range pageResult.Sources {
		data = append(data, toSourceSummaryResponse(source))
	}
	writeSourcePage(response, data, pageResult.Page, pageResult.PageSize, pageResult.TotalItems)
}

func toSourceResponse(source *Source) sourceResponse {
	return sourceResponse{
		ID:               source.ID(),
		Kind:             source.Kind(),
		SpaceID:          nullableSpaceID(source.SpaceID()),
		Title:            source.Title(),
		CaptureNote:      source.CaptureNote(),
		Content:          source.Content(),
		ProcessingStatus: source.ProcessingStatus(),
		CreatedAt:        source.CreatedAt().UTC().Format("2006-01-02T15:04:05.000000000Z07:00"),
		UpdatedAt:        source.UpdatedAt().UTC().Format("2006-01-02T15:04:05.000000000Z07:00"),
	}
}

func toSourceSummaryResponse(source SourceSummary) sourceSummaryResponse {
	return sourceSummaryResponse{
		ID:               source.ID,
		Kind:             source.Kind,
		SpaceID:          nullableSpaceID(source.SpaceID),
		Title:            source.Title,
		CaptureNote:      source.CaptureNote,
		ProcessingStatus: source.ProcessingStatus,
		CreatedAt:        source.CreatedAt.UTC().Format("2006-01-02T15:04:05.000000000Z07:00"),
		UpdatedAt:        source.UpdatedAt.UTC().Format("2006-01-02T15:04:05.000000000Z07:00"),
	}
}

func nullableSpaceID(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func sourcePagination(response http.ResponseWriter, request *http.Request) (int, int, bool) {
	page, err := sourcePaginationValue(request.URL.Query().Get("page"), 1, 1, 1_000_000)
	if err != nil {
		httpapi.WriteError(response, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "page must be a positive integer")
		return 0, 0, false
	}
	pageSize, err := sourcePaginationValue(request.URL.Query().Get("pageSize"), 50, 1, 100)
	if err != nil {
		httpapi.WriteError(response, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "pageSize must be between 1 and 100")
		return 0, 0, false
	}
	return page, pageSize, true
}

func sourcePaginationValue(raw string, fallback, minimum, maximum int) (int, error) {
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < minimum || value > maximum {
		return 0, errors.New("pagination value is outside the allowed range")
	}
	return value, nil
}

func writeSourcePage(response http.ResponseWriter, data any, page, pageSize, totalItems int) {
	totalPages := 0
	if totalItems > 0 {
		totalPages = (totalItems + pageSize - 1) / pageSize
	}
	_ = httpapi.WriteJSON(response, http.StatusOK, map[string]any{
		"data": data,
		"pagination": map[string]int{
			"page": page, "pageSize": pageSize, "totalItems": totalItems, "totalPages": totalPages,
		},
	})
}

func writeSourceDecodeError(response http.ResponseWriter, err error) {
	if errors.Is(err, httpapi.ErrBodyTooLarge) {
		httpapi.WriteError(response, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "request body is too large")
		return
	}
	httpapi.WriteError(response, http.StatusBadRequest, "INVALID_REQUEST", "request body must contain one valid JSON object")
}

func writeSourceServiceError(response http.ResponseWriter, err error, operation string) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrSourceIDRequired) || errors.Is(err, ErrSourceTitleRequired) {
		httpapi.WriteError(response, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error())
		return true
	}
	if errors.Is(err, ErrSourceNotFound) {
		httpapi.WriteError(response, http.StatusNotFound, "SOURCE_NOT_FOUND", "Learning Source not found")
		return true
	}
	httpapi.WriteError(response, http.StatusInternalServerError, "INTERNAL_ERROR", "could not "+operation+" Learning Source")
	return true
}
