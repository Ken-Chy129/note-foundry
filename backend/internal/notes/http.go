package notes

import (
	"errors"
	"net/http"

	"github.com/Ken-Chy129/note-foundry/backend/internal/knowledge"
	"github.com/Ken-Chy129/note-foundry/backend/internal/platform/httpapi"
)

const maxNoteRequestBytes = 2 * 1024 * 1024

type OwnerMiddleware func(http.Handler) http.Handler

type HTTPHandler struct {
	service      *Service
	requireOwner OwnerMiddleware
}

func NewHTTPHandler(service *Service, requireOwner OwnerMiddleware) *HTTPHandler {
	return &HTTPHandler{service: service, requireOwner: requireOwner}
}

func (handler *HTTPHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle("POST /api/v1/notes", handler.requireOwner(http.HandlerFunc(handler.createNote)))
	mux.Handle("GET /api/v1/notes/{noteId}", handler.requireOwner(http.HandlerFunc(handler.getNote)))
	mux.Handle("PATCH /api/v1/notes/{noteId}", handler.requireOwner(http.HandlerFunc(handler.autosaveNote)))
	mux.Handle("POST /api/v1/notes/{noteId}/publish", handler.requireOwner(http.HandlerFunc(handler.publishNote)))
}

type noteResponse struct {
	ID          string                    `json:"id"`
	SpaceID     string                    `json:"spaceId"`
	DirectoryID *string                   `json:"directoryId"`
	Title       string                    `json:"title"`
	Slug        string                    `json:"slug"`
	Markdown    string                    `json:"markdown"`
	Version     int64                     `json:"version"`
	Published   *publishedContentResponse `json:"published"`
}

type publishedContentResponse struct {
	Title       string `json:"title"`
	Slug        string `json:"slug"`
	Markdown    string `json:"markdown"`
	PublishedAt string `json:"publishedAt"`
}

func (handler *HTTPHandler) createNote(response http.ResponseWriter, request *http.Request) {
	var input struct {
		SpaceID     string  `json:"spaceId"`
		DirectoryID *string `json:"directoryId"`
		Title       string  `json:"title"`
		Markdown    string  `json:"markdown"`
	}
	if err := httpapi.DecodeJSON(request.Body, maxNoteRequestBytes, &input); err != nil {
		writeNoteDecodeError(response, err)
		return
	}
	directoryID := ""
	if input.DirectoryID != nil {
		directoryID = *input.DirectoryID
	}
	note, err := handler.service.CreateNote(request.Context(), input.SpaceID, directoryID, input.Title, input.Markdown)
	if writeNoteServiceError(response, err, "create") {
		return
	}
	response.Header().Set("Location", "/api/v1/notes/"+note.ID())
	_ = httpapi.WriteJSON(response, http.StatusCreated, toNoteResponse(note))
}

func (handler *HTTPHandler) getNote(response http.ResponseWriter, request *http.Request) {
	note, err := handler.service.GetNote(request.Context(), request.PathValue("noteId"))
	if writeNoteServiceError(response, err, "load") {
		return
	}
	_ = httpapi.WriteJSON(response, http.StatusOK, toNoteResponse(note))
}

func (handler *HTTPHandler) autosaveNote(response http.ResponseWriter, request *http.Request) {
	var input struct {
		ExpectedVersion int64  `json:"expectedVersion"`
		Title           string `json:"title"`
		Markdown        string `json:"markdown"`
	}
	if err := httpapi.DecodeJSON(request.Body, maxNoteRequestBytes, &input); err != nil {
		writeNoteDecodeError(response, err)
		return
	}
	note, err := handler.service.Autosave(request.Context(), request.PathValue("noteId"), input.ExpectedVersion, input.Title, input.Markdown)
	if writeNoteServiceError(response, err, "save") {
		return
	}
	_ = httpapi.WriteJSON(response, http.StatusOK, toNoteResponse(note))
}

func (handler *HTTPHandler) publishNote(response http.ResponseWriter, request *http.Request) {
	var input struct {
		ExpectedVersion int64 `json:"expectedVersion"`
	}
	if err := httpapi.DecodeJSON(request.Body, maxNoteRequestBytes, &input); err != nil {
		writeNoteDecodeError(response, err)
		return
	}
	note, err := handler.service.Publish(request.Context(), request.PathValue("noteId"), input.ExpectedVersion)
	if writeNoteServiceError(response, err, "publish") {
		return
	}
	_ = httpapi.WriteJSON(response, http.StatusOK, toNoteResponse(note))
}

func toNoteResponse(note *Note) noteResponse {
	var directoryID *string
	if note.DirectoryID() != "" {
		value := note.DirectoryID()
		directoryID = &value
	}
	var published *publishedContentResponse
	if note.Published() != nil {
		published = &publishedContentResponse{
			Title:       note.Published().Title,
			Slug:        note.Published().Slug,
			Markdown:    note.Published().Markdown,
			PublishedAt: note.Published().PublishedAt.UTC().Format("2006-01-02T15:04:05.000000000Z07:00"),
		}
	}
	return noteResponse{
		ID:          note.ID(),
		SpaceID:     note.SpaceID(),
		DirectoryID: directoryID,
		Title:       note.Title(),
		Slug:        note.Slug(),
		Markdown:    note.Markdown(),
		Version:     note.Version(),
		Published:   published,
	}
}

func writeNoteServiceError(response http.ResponseWriter, err error, operation string) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrNoteIDRequired) || errors.Is(err, ErrNoteSpaceIDRequired) || errors.Is(err, ErrNoteTitleRequired) || errors.Is(err, ErrInvalidNoteDirectory) {
		httpapi.WriteError(response, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error())
		return true
	}
	if errors.Is(err, ErrVersionConflict) {
		httpapi.WriteError(response, http.StatusConflict, "NOTE_VERSION_CONFLICT", "Learning Note was changed by another save")
		return true
	}
	if errors.Is(err, ErrPrivateNotePublish) {
		httpapi.WriteError(response, http.StatusConflict, "PRIVATE_NOTE_NOT_PUBLISHABLE", err.Error())
		return true
	}
	if errors.Is(err, ErrNoteNotFound) {
		httpapi.WriteError(response, http.StatusNotFound, "NOTE_NOT_FOUND", "Learning Note not found")
		return true
	}
	if errors.Is(err, knowledge.ErrSpaceNotFound) {
		httpapi.WriteError(response, http.StatusNotFound, "SPACE_NOT_FOUND", "Knowledge Space not found")
		return true
	}
	if errors.Is(err, knowledge.ErrDirectoryNotFound) {
		httpapi.WriteError(response, http.StatusNotFound, "DIRECTORY_NOT_FOUND", "directory not found")
		return true
	}
	httpapi.WriteError(response, http.StatusInternalServerError, "INTERNAL_ERROR", "could not "+operation+" Learning Note")
	return true
}

func writeNoteDecodeError(response http.ResponseWriter, err error) {
	if errors.Is(err, httpapi.ErrBodyTooLarge) {
		httpapi.WriteError(response, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "request body is too large")
		return
	}
	httpapi.WriteError(response, http.StatusBadRequest, "INVALID_REQUEST", "request body must contain one valid JSON object")
}
