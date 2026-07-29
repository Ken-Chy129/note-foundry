package knowledge

import (
	"context"
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
	mux.Handle("GET /api/v1/spaces/{spaceId}/directories", handler.requireOwner(http.HandlerFunc(handler.listDirectories)))
	mux.Handle("POST /api/v1/spaces/{spaceId}/directories", handler.requireOwner(http.HandlerFunc(handler.createDirectory)))
	mux.Handle("PATCH /api/v1/directories/{directoryId}", handler.requireOwner(http.HandlerFunc(handler.renameDirectory)))
	mux.Handle("POST /api/v1/directories/{directoryId}/move", handler.requireOwner(http.HandlerFunc(handler.moveDirectory)))
	mux.Handle("GET /api/v1/tags", handler.requireOwner(http.HandlerFunc(handler.listTags)))
	mux.Handle("POST /api/v1/tags", handler.requireOwner(http.HandlerFunc(handler.createTag)))
	mux.Handle("PATCH /api/v1/tags/{tagId}", handler.requireOwner(http.HandlerFunc(handler.renameTag)))
	mux.Handle("POST /api/v1/tags/{tagId}/merge", handler.requireOwner(http.HandlerFunc(handler.mergeTag)))
	mux.Handle("GET /api/v1/notes/{noteId}/tags", handler.requireOwner(http.HandlerFunc(handler.listNoteTags)))
	mux.Handle("PUT /api/v1/notes/{noteId}/tags", handler.requireOwner(http.HandlerFunc(handler.setNoteTags)))
	mux.Handle("GET /api/v1/notes/{noteId}/links", handler.requireOwner(http.HandlerFunc(handler.listCurrentForwardLinks)))
	mux.Handle("GET /api/v1/notes/{noteId}/backlinks", handler.requireOwner(http.HandlerFunc(handler.listCurrentBacklinks)))
	mux.HandleFunc("GET /api/v1/public/spaces", handler.listPublicSpaces)
	mux.HandleFunc("GET /api/v1/public/spaces/{spaceId}/directories", handler.listPublicDirectories)
	mux.HandleFunc("GET /api/v1/public/notes/{noteId}/links", handler.listPublishedForwardLinks)
	mux.HandleFunc("GET /api/v1/public/notes/{noteId}/backlinks", handler.listPublishedBacklinks)
}

type spaceResponse struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Visibility Visibility `json:"visibility"`
}

type directoryResponse struct {
	ID       string  `json:"id"`
	SpaceID  string  `json:"spaceId"`
	ParentID *string `json:"parentId"`
	Name     string  `json:"name"`
}

type tagResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type linkedNoteResponse struct {
	ID      string `json:"id"`
	SpaceID string `json:"spaceId"`
	Title   string `json:"title"`
	Slug    string `json:"slug"`
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

func (handler *HTTPHandler) listPublicSpaces(response http.ResponseWriter, request *http.Request) {
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
	pageResult, err := handler.service.ListPublicSpaces(request.Context(), page, pageSize)
	if err != nil {
		httpapi.WriteError(response, http.StatusInternalServerError, "INTERNAL_ERROR", "could not list public Knowledge Spaces")
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

func (handler *HTTPHandler) createDirectory(response http.ResponseWriter, request *http.Request) {
	var input struct {
		Name     string  `json:"name"`
		ParentID *string `json:"parentId"`
	}
	if err := httpapi.DecodeJSON(request.Body, maxKnowledgeRequestBytes, &input); err != nil {
		writeDecodeError(response, err)
		return
	}
	parentID := ""
	if input.ParentID != nil {
		parentID = *input.ParentID
	}

	directory, err := handler.service.CreateDirectory(request.Context(), request.PathValue("spaceId"), parentID, input.Name)
	if writeDirectoryServiceError(response, err, "create") {
		return
	}
	response.Header().Set("Location", "/api/v1/directories/"+directory.ID())
	_ = httpapi.WriteJSON(response, http.StatusCreated, toDirectoryResponse(directory))
}

func (handler *HTTPHandler) listDirectories(response http.ResponseWriter, request *http.Request) {
	directories, err := handler.service.ListDirectories(request.Context(), request.PathValue("spaceId"))
	if errors.Is(err, ErrSpaceNotFound) {
		httpapi.WriteError(response, http.StatusNotFound, "SPACE_NOT_FOUND", "Knowledge Space not found")
		return
	}
	if err != nil {
		httpapi.WriteError(response, http.StatusInternalServerError, "INTERNAL_ERROR", "could not list directories")
		return
	}
	data := make([]directoryResponse, 0, len(directories))
	for _, directory := range directories {
		data = append(data, toDirectoryResponse(directory))
	}
	_ = httpapi.WriteJSON(response, http.StatusOK, map[string]any{"data": data})
}

func (handler *HTTPHandler) listPublicDirectories(response http.ResponseWriter, request *http.Request) {
	directories, err := handler.service.ListPublicDirectories(request.Context(), request.PathValue("spaceId"))
	if errors.Is(err, ErrSpaceNotFound) {
		httpapi.WriteError(response, http.StatusNotFound, "SPACE_NOT_FOUND", "public Knowledge Space not found")
		return
	}
	if err != nil {
		httpapi.WriteError(response, http.StatusInternalServerError, "INTERNAL_ERROR", "could not list public directories")
		return
	}
	data := make([]directoryResponse, 0, len(directories))
	for _, directory := range directories {
		data = append(data, toDirectoryResponse(directory))
	}
	_ = httpapi.WriteJSON(response, http.StatusOK, map[string]any{"data": data})
}

func (handler *HTTPHandler) renameDirectory(response http.ResponseWriter, request *http.Request) {
	var input struct {
		Name string `json:"name"`
	}
	if err := httpapi.DecodeJSON(request.Body, maxKnowledgeRequestBytes, &input); err != nil {
		writeDecodeError(response, err)
		return
	}
	directory, err := handler.service.RenameDirectory(request.Context(), request.PathValue("directoryId"), input.Name)
	if writeDirectoryServiceError(response, err, "rename") {
		return
	}
	_ = httpapi.WriteJSON(response, http.StatusOK, toDirectoryResponse(directory))
}

func (handler *HTTPHandler) moveDirectory(response http.ResponseWriter, request *http.Request) {
	var input struct {
		ParentID *string `json:"parentId"`
	}
	if err := httpapi.DecodeJSON(request.Body, maxKnowledgeRequestBytes, &input); err != nil {
		writeDecodeError(response, err)
		return
	}
	parentID := ""
	if input.ParentID != nil {
		parentID = *input.ParentID
	}
	directory, err := handler.service.MoveDirectory(request.Context(), request.PathValue("directoryId"), parentID)
	if writeDirectoryServiceError(response, err, "move") {
		return
	}
	_ = httpapi.WriteJSON(response, http.StatusOK, toDirectoryResponse(directory))
}

func (handler *HTTPHandler) createTag(response http.ResponseWriter, request *http.Request) {
	var input struct {
		Name string `json:"name"`
	}
	if err := httpapi.DecodeJSON(request.Body, maxKnowledgeRequestBytes, &input); err != nil {
		writeDecodeError(response, err)
		return
	}
	tag, err := handler.service.CreateTag(request.Context(), input.Name)
	if writeTagServiceError(response, err, "create") {
		return
	}
	response.Header().Set("Location", "/api/v1/tags/"+tag.ID())
	_ = httpapi.WriteJSON(response, http.StatusCreated, toTagResponse(tag))
}

func (handler *HTTPHandler) listTags(response http.ResponseWriter, request *http.Request) {
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
	pageResult, err := handler.service.ListTags(request.Context(), page, pageSize)
	if err != nil {
		httpapi.WriteError(response, http.StatusInternalServerError, "INTERNAL_ERROR", "could not list tags")
		return
	}
	data := make([]tagResponse, 0, len(pageResult.Tags))
	for _, tag := range pageResult.Tags {
		data = append(data, toTagResponse(tag))
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

func (handler *HTTPHandler) renameTag(response http.ResponseWriter, request *http.Request) {
	var input struct {
		Name string `json:"name"`
	}
	if err := httpapi.DecodeJSON(request.Body, maxKnowledgeRequestBytes, &input); err != nil {
		writeDecodeError(response, err)
		return
	}
	tag, err := handler.service.RenameTag(request.Context(), request.PathValue("tagId"), input.Name)
	if writeTagServiceError(response, err, "rename") {
		return
	}
	_ = httpapi.WriteJSON(response, http.StatusOK, toTagResponse(tag))
}

func (handler *HTTPHandler) setNoteTags(response http.ResponseWriter, request *http.Request) {
	var input struct {
		TagIDs []string `json:"tagIds"`
	}
	if err := httpapi.DecodeJSON(request.Body, maxKnowledgeRequestBytes, &input); err != nil {
		writeDecodeError(response, err)
		return
	}
	tags, err := handler.service.SetNoteTags(request.Context(), request.PathValue("noteId"), input.TagIDs)
	if writeTagServiceError(response, err, "set Learning Note") {
		return
	}
	data := make([]tagResponse, 0, len(tags))
	for _, tag := range tags {
		data = append(data, toTagResponse(tag))
	}
	_ = httpapi.WriteJSON(response, http.StatusOK, map[string]any{"data": data})
}

func (handler *HTTPHandler) listNoteTags(response http.ResponseWriter, request *http.Request) {
	tags, err := handler.service.ListNoteTags(request.Context(), request.PathValue("noteId"))
	if writeTagServiceError(response, err, "list Learning Note") {
		return
	}
	data := make([]tagResponse, 0, len(tags))
	for _, tag := range tags {
		data = append(data, toTagResponse(tag))
	}
	_ = httpapi.WriteJSON(response, http.StatusOK, map[string]any{"data": data})
}

func (handler *HTTPHandler) mergeTag(response http.ResponseWriter, request *http.Request) {
	var input struct {
		TargetTagID string `json:"targetTagId"`
	}
	if err := httpapi.DecodeJSON(request.Body, maxKnowledgeRequestBytes, &input); err != nil {
		writeDecodeError(response, err)
		return
	}
	tag, err := handler.service.MergeTag(request.Context(), request.PathValue("tagId"), input.TargetTagID)
	if writeTagServiceError(response, err, "merge") {
		return
	}
	_ = httpapi.WriteJSON(response, http.StatusOK, toTagResponse(tag))
}

func (handler *HTTPHandler) listCurrentForwardLinks(response http.ResponseWriter, request *http.Request) {
	handler.writeLinks(response, request, handler.service.ListCurrentForwardLinks)
}

func (handler *HTTPHandler) listCurrentBacklinks(response http.ResponseWriter, request *http.Request) {
	handler.writeLinks(response, request, handler.service.ListCurrentBacklinks)
}

func (handler *HTTPHandler) listPublishedForwardLinks(response http.ResponseWriter, request *http.Request) {
	handler.writeLinks(response, request, handler.service.ListPublishedForwardLinks)
}

func (handler *HTTPHandler) listPublishedBacklinks(response http.ResponseWriter, request *http.Request) {
	handler.writeLinks(response, request, handler.service.ListPublishedBacklinks)
}

func (handler *HTTPHandler) writeLinks(response http.ResponseWriter, request *http.Request, query func(context.Context, string) ([]LinkedNote, error)) {
	links, err := query(request.Context(), request.PathValue("noteId"))
	if writeLinkServiceError(response, err) {
		return
	}
	data := make([]linkedNoteResponse, 0, len(links))
	for _, link := range links {
		data = append(data, linkedNoteResponse{ID: link.ID, SpaceID: link.SpaceID, Title: link.Title, Slug: link.Slug})
	}
	_ = httpapi.WriteJSON(response, http.StatusOK, map[string]any{"data": data})
}

func toSpaceResponse(space *Space) spaceResponse {
	return spaceResponse{ID: space.ID(), Name: space.Name(), Visibility: space.Visibility()}
}

func toDirectoryResponse(directory *Directory) directoryResponse {
	var parentID *string
	if directory.ParentID() != "" {
		value := directory.ParentID()
		parentID = &value
	}
	return directoryResponse{
		ID:       directory.ID(),
		SpaceID:  directory.SpaceID(),
		ParentID: parentID,
		Name:     directory.Name(),
	}
}

func toTagResponse(tag *Tag) tagResponse {
	return tagResponse{ID: tag.ID(), Name: tag.Name()}
}

func writeDirectoryServiceError(response http.ResponseWriter, err error, operation string) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrDirectoryNameRequired) || errors.Is(err, ErrDirectorySelfParent) || errors.Is(err, ErrDirectoryWrongSpace) || errors.Is(err, ErrDirectoryParentInvalid) {
		httpapi.WriteError(response, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error())
		return true
	}
	if errors.Is(err, ErrDirectoryCycle) {
		httpapi.WriteError(response, http.StatusConflict, "DIRECTORY_CYCLE", "directory move would create a cycle")
		return true
	}
	if errors.Is(err, ErrDirectoryNameConflict) {
		httpapi.WriteError(response, http.StatusConflict, "DIRECTORY_NAME_CONFLICT", "a directory with this name already exists under the target parent")
		return true
	}
	if errors.Is(err, ErrDirectoryNotFound) {
		httpapi.WriteError(response, http.StatusNotFound, "DIRECTORY_NOT_FOUND", "directory not found")
		return true
	}
	if errors.Is(err, ErrSpaceNotFound) {
		httpapi.WriteError(response, http.StatusNotFound, "SPACE_NOT_FOUND", "Knowledge Space not found")
		return true
	}
	httpapi.WriteError(response, http.StatusInternalServerError, "INTERNAL_ERROR", "could not "+operation+" directory")
	return true
}

func writeTagServiceError(response http.ResponseWriter, err error, operation string) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrTagNameRequired) {
		httpapi.WriteError(response, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error())
		return true
	}
	if errors.Is(err, ErrTagMergeSame) {
		httpapi.WriteError(response, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error())
		return true
	}
	if errors.Is(err, ErrTagNameConflict) {
		httpapi.WriteError(response, http.StatusConflict, "TAG_NAME_CONFLICT", "a tag with this name already exists")
		return true
	}
	if errors.Is(err, ErrTagNotFound) {
		httpapi.WriteError(response, http.StatusNotFound, "TAG_NOT_FOUND", "tag not found")
		return true
	}
	if errors.Is(err, ErrTaggableNoteNotFound) {
		httpapi.WriteError(response, http.StatusNotFound, "NOTE_NOT_FOUND", "Learning Note not found")
		return true
	}
	httpapi.WriteError(response, http.StatusInternalServerError, "INTERNAL_ERROR", "could not "+operation+" tag")
	return true
}

func writeLinkServiceError(response http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrLinkedNoteNotFound) {
		httpapi.WriteError(response, http.StatusNotFound, "NOTE_NOT_FOUND", "Learning Note not found")
		return true
	}
	httpapi.WriteError(response, http.StatusInternalServerError, "INTERNAL_ERROR", "could not list Note Links")
	return true
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
