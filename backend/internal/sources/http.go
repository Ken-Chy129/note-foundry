package sources

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/Ken-Chy129/note-foundry/backend/internal/platform/httpapi"
)

const maxSourceRequestBytes = 2 * 1024 * 1024

type OwnerMiddleware func(http.Handler) http.Handler

type sourceService interface {
	CreateManualSource(context.Context, CreateManualSourceInput) (*Source, error)
	CreateURLSource(context.Context, CreateURLSourceInput) (CreateURLSourceResult, error)
	GetSource(context.Context, string) (*Source, error)
	ListSources(context.Context, ListFilter) (SourcePage, error)
	OrganizeSource(context.Context, string, string) (*Source, error)
	RetryURLExtraction(context.Context, string) (*Source, error)
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
	OriginalURL      *string          `json:"originalUrl"`
	NormalizedURL    *string          `json:"normalizedUrl"`
	ProcessingStatus ProcessingStatus `json:"processingStatus"`
	FailureMessage   *string          `json:"failureMessage"`
	CreatedAt        string           `json:"createdAt"`
	UpdatedAt        string           `json:"updatedAt"`
}

type sourceSummaryResponse struct {
	ID               string           `json:"id"`
	Kind             Kind             `json:"kind"`
	SpaceID          *string          `json:"spaceId"`
	Title            string           `json:"title"`
	CaptureNote      string           `json:"captureNote"`
	OriginalURL      *string          `json:"originalUrl"`
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
	mux.Handle("PATCH /api/v1/sources/{sourceId}", handler.requireOwner(http.HandlerFunc(handler.organizeSource)))
	mux.Handle("POST /api/v1/sources/{sourceId}/retry-extraction", handler.requireOwner(http.HandlerFunc(handler.retryURLExtraction)))
}

func (handler *HTTPHandler) retryURLExtraction(response http.ResponseWriter, request *http.Request) {
	source, err := handler.service.RetryURLExtraction(request.Context(), request.PathValue("sourceId"))
	if writeSourceServiceError(response, err, "retry extraction for") {
		return
	}
	_ = httpapi.WriteJSON(response, http.StatusOK, toSourceResponse(source))
}

func (handler *HTTPHandler) createSource(response http.ResponseWriter, request *http.Request) {
	var input struct {
		Kind        Kind   `json:"kind"`
		Title       string `json:"title"`
		CaptureNote string `json:"captureNote"`
		Content     string `json:"content"`
		OriginalURL string `json:"originalUrl"`
	}
	if err := httpapi.DecodeJSON(request.Body, maxSourceRequestBytes, &input); err != nil {
		writeSourceDecodeError(response, err)
		return
	}
	if input.Kind == KindURL {
		result, err := handler.service.CreateURLSource(request.Context(), CreateURLSourceInput{
			Title:       input.Title,
			CaptureNote: input.CaptureNote,
			OriginalURL: input.OriginalURL,
		})
		if writeSourceServiceError(response, err, "create") {
			return
		}
		response.Header().Set("Location", "/api/v1/sources/"+result.Source.ID())
		status := http.StatusOK
		if result.Created {
			status = http.StatusCreated
		}
		_ = httpapi.WriteJSON(response, status, toSourceResponse(result.Source))
		return
	}
	if input.Kind != KindManual {
		httpapi.WriteError(response, http.StatusUnprocessableEntity, "SOURCE_KIND_NOT_SUPPORTED", "only manual and URL Learning Sources are supported in this release slice")
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

func (handler *HTTPHandler) organizeSource(response http.ResponseWriter, request *http.Request) {
	var input struct {
		SpaceID json.RawMessage `json:"spaceId"`
	}
	if err := httpapi.DecodeJSON(request.Body, maxSourceRequestBytes, &input); err != nil {
		writeSourceDecodeError(response, err)
		return
	}
	if len(input.SpaceID) == 0 {
		httpapi.WriteError(response, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "spaceId is required and may be null")
		return
	}
	spaceID := ""
	if !bytes.Equal(bytes.TrimSpace(input.SpaceID), []byte("null")) {
		if err := json.Unmarshal(input.SpaceID, &spaceID); err != nil || strings.TrimSpace(spaceID) == "" {
			httpapi.WriteError(response, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "spaceId must be a Knowledge Space id or null")
			return
		}
	}
	source, err := handler.service.OrganizeSource(request.Context(), request.PathValue("sourceId"), spaceID)
	if writeSourceServiceError(response, err, "organize") {
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
	spaceID := strings.TrimSpace(request.URL.Query().Get("spaceId"))
	if inboxOnly && spaceID != "" {
		httpapi.WriteError(response, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "inbox and spaceId filters cannot be combined")
		return
	}
	pageResult, err := handler.service.ListSources(request.Context(), ListFilter{InboxOnly: inboxOnly, SpaceID: spaceID, Page: page, PageSize: pageSize})
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
		OriginalURL:      nullableSourceString(source.OriginalURL()),
		NormalizedURL:    nullableSourceString(source.NormalizedURL()),
		ProcessingStatus: source.ProcessingStatus(),
		FailureMessage:   nullableSourceString(source.FailureMessage()),
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
		OriginalURL:      nullableSourceString(source.OriginalURL),
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

func nullableSourceString(value string) *string {
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
	if errors.Is(err, ErrSourceIDRequired) || errors.Is(err, ErrSourceTitleRequired) || errors.Is(err, ErrSourceURLInvalid) {
		httpapi.WriteError(response, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error())
		return true
	}
	if errors.Is(err, ErrSourceNotFound) {
		httpapi.WriteError(response, http.StatusNotFound, "SOURCE_NOT_FOUND", "Learning Source not found")
		return true
	}
	if errors.Is(err, ErrSourceSpaceNotFound) {
		httpapi.WriteError(response, http.StatusNotFound, "SPACE_NOT_FOUND", "Knowledge Space not found")
		return true
	}
	if errors.Is(err, ErrSourceExtractionUnsupported) || errors.Is(err, ErrSourceExtractionNotFailed) {
		httpapi.WriteError(response, http.StatusConflict, "SOURCE_EXTRACTION_NOT_RETRYABLE", err.Error())
		return true
	}
	httpapi.WriteError(response, http.StatusInternalServerError, "INTERNAL_ERROR", "could not "+operation+" Learning Source")
	return true
}
