package knowledge

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Ken-Chy129/note-foundry/backend/internal/platform/httpapi"
)

const maxKnowledgeRequestBytes = 16 * 1024

type OwnerMiddleware func(http.Handler) http.Handler

type HTTPHandler struct {
	service      *Service
	requireOwner OwnerMiddleware
}

func NewHTTPHandler(service *Service, requireOwner OwnerMiddleware) *HTTPHandler {
	return &HTTPHandler{service: service, requireOwner: requireOwner}
}

func (handler *HTTPHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/spaces", handler.requireOwner(http.HandlerFunc(handler.listSpaces)))
	mux.Handle("POST /api/v1/spaces", handler.requireOwner(http.HandlerFunc(handler.createSpace)))
	mux.Handle("PATCH /api/v1/spaces/{spaceId}", handler.requireOwner(http.HandlerFunc(handler.renameSpace)))
}

type spaceResponse struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Visibility Visibility `json:"visibility"`
}

func (handler *HTTPHandler) createSpace(response http.ResponseWriter, request *http.Request) {
	var input struct {
		Name       string     `json:"name"`
		Visibility Visibility `json:"visibility"`
	}
	if err := httpapi.DecodeJSON(request.Body, maxKnowledgeRequestBytes, &input); err != nil {
		writeDecodeError(response, err)
		return
	}

	space, err := handler.service.CreateSpace(request.Context(), input.Name, input.Visibility)
	if errors.Is(err, ErrSpaceIDRequired) || errors.Is(err, ErrSpaceNameRequired) || errors.Is(err, ErrInvalidSpaceVisibility) {
		httpapi.WriteError(response, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error())
		return
	}
	if errors.Is(err, ErrSpaceNameConflict) {
		httpapi.WriteError(response, http.StatusConflict, "SPACE_NAME_CONFLICT", "a Knowledge Space with this name already exists")
		return
	}
	if err != nil {
		httpapi.WriteError(response, http.StatusInternalServerError, "INTERNAL_ERROR", "could not create Knowledge Space")
		return
	}

	response.Header().Set("Location", "/api/v1/spaces/"+space.ID())
	_ = httpapi.WriteJSON(response, http.StatusCreated, toSpaceResponse(space))
}

func (handler *HTTPHandler) listSpaces(response http.ResponseWriter, request *http.Request) {
	page, err := paginationValue(request, "page", 1, 1, 1_000_000)
	if err != nil {
		httpapi.WriteError(response, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "page must be a positive integer")
		return
	}
	pageSize, err := paginationValue(request, "pageSize", 50, 1, 100)
	if err != nil {
		httpapi.WriteError(response, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "pageSize must be between 1 and 100")
		return
	}

	pageResult, err := handler.service.ListSpaces(request.Context(), page, pageSize)
	if err != nil {
		httpapi.WriteError(response, http.StatusInternalServerError, "INTERNAL_ERROR", "could not list Knowledge Spaces")
		return
	}
	data := make([]spaceResponse, 0, len(pageResult.Spaces))
	for _, space := range pageResult.Spaces {
		data = append(data, toSpaceResponse(space))
	}
	totalPages := 0
	if pageResult.TotalItems > 0 {
		totalPages = (pageResult.TotalItems + pageResult.PageSize - 1) / pageResult.PageSize
	}
	_ = httpapi.WriteJSON(response, http.StatusOK, map[string]any{
		"data": data,
		"pagination": map[string]int{
			"page":       pageResult.Page,
			"pageSize":   pageResult.PageSize,
			"totalItems": pageResult.TotalItems,
			"totalPages": totalPages,
		},
	})
}

func (handler *HTTPHandler) renameSpace(response http.ResponseWriter, request *http.Request) {
	var input struct {
		Name string `json:"name"`
	}
	if err := httpapi.DecodeJSON(request.Body, maxKnowledgeRequestBytes, &input); err != nil {
		writeDecodeError(response, err)
		return
	}

	space, err := handler.service.RenameSpace(request.Context(), request.PathValue("spaceId"), input.Name)
	if errors.Is(err, ErrSpaceNameRequired) {
		httpapi.WriteError(response, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error())
		return
	}
	if errors.Is(err, ErrSpaceNotFound) {
		httpapi.WriteError(response, http.StatusNotFound, "SPACE_NOT_FOUND", "Knowledge Space not found")
		return
	}
	if errors.Is(err, ErrSpaceNameConflict) {
		httpapi.WriteError(response, http.StatusConflict, "SPACE_NAME_CONFLICT", "a Knowledge Space with this name already exists")
		return
	}
	if err != nil {
		httpapi.WriteError(response, http.StatusInternalServerError, "INTERNAL_ERROR", "could not rename Knowledge Space")
		return
	}
	_ = httpapi.WriteJSON(response, http.StatusOK, toSpaceResponse(space))
}

func toSpaceResponse(space *Space) spaceResponse {
	return spaceResponse{ID: space.ID(), Name: space.Name(), Visibility: space.Visibility()}
}

func writeDecodeError(response http.ResponseWriter, err error) {
	if errors.Is(err, httpapi.ErrBodyTooLarge) {
		httpapi.WriteError(response, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "request body is too large")
		return
	}
	httpapi.WriteError(response, http.StatusBadRequest, "INVALID_JSON", "request body must contain one valid JSON object")
}

func paginationValue(request *http.Request, key string, fallback, minimum, maximum int) (int, error) {
	value := request.URL.Query().Get(key)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < minimum || parsed > maximum {
		return 0, errors.New("invalid pagination value")
	}
	return parsed, nil
}
