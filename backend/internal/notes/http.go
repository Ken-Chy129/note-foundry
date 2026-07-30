package notes

import (
	"errors"
	"net/http"
	"strconv"

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
	mux.Handle("GET /api/v1/notes", handler.requireOwner(http.HandlerFunc(handler.listNotes)))
	mux.Handle("POST /api/v1/notes", handler.requireOwner(http.HandlerFunc(handler.createNote)))
	mux.Handle("GET /api/v1/notes/{noteId}", handler.requireOwner(http.HandlerFunc(handler.getNote)))
	mux.Handle("PATCH /api/v1/notes/{noteId}", handler.requireOwner(http.HandlerFunc(handler.autosaveNote)))
	mux.Handle("POST /api/v1/notes/{noteId}/publish", handler.requireOwner(http.HandlerFunc(handler.publishNote)))
	mux.Handle("POST /api/v1/notes/{noteId}/move", handler.requireOwner(http.HandlerFunc(handler.moveNote)))
	mux.Handle("GET /api/v1/notes/{noteId}/revisions", handler.requireOwner(http.HandlerFunc(handler.listRevisions)))
	mux.Handle("POST /api/v1/notes/{noteId}/revisions", handler.requireOwner(http.HandlerFunc(handler.createCheckpoint)))
	mux.Handle("POST /api/v1/notes/{noteId}/revisions/{revisionId}/restore", handler.requireOwner(http.HandlerFunc(handler.restoreRevision)))
	mux.Handle("POST /api/v1/notes/{noteId}/trash", handler.requireOwner(http.HandlerFunc(handler.trashNote)))
	mux.Handle("GET /api/v1/trash/notes", handler.requireOwner(http.HandlerFunc(handler.listTrash)))
	mux.Handle("POST /api/v1/trash/notes/{noteId}/restore", handler.requireOwner(http.HandlerFunc(handler.restoreFromTrash)))
	mux.Handle("DELETE /api/v1/trash/notes/{noteId}", handler.requireOwner(http.HandlerFunc(handler.deleteTrashedNote)))
	mux.HandleFunc("GET /api/v1/public/spaces/{spaceId}/notes", handler.listPublishedNotes)
	mux.HandleFunc("GET /api/v1/public/notes/{noteId}", handler.getPublishedNote)
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
	UpdatedAt   string                    `json:"updatedAt"`
}

type noteSummaryResponse struct {
	ID          string  `json:"id"`
	SpaceID     string  `json:"spaceId"`
	DirectoryID *string `json:"directoryId"`
	Title       string  `json:"title"`
	Slug        string  `json:"slug"`
	Version     int64   `json:"version"`
	IsPublished bool    `json:"isPublished"`
	UpdatedAt   string  `json:"updatedAt"`
}

type publishedContentResponse struct {
	Title       string `json:"title"`
	Slug        string `json:"slug"`
	Markdown    string `json:"markdown"`
	PublishedAt string `json:"publishedAt"`
}

type revisionResponse struct {
	ID        string         `json:"id"`
	NoteID    string         `json:"noteId"`
	Title     string         `json:"title"`
	Slug      string         `json:"slug"`
	Markdown  string         `json:"markdown"`
	Reason    RevisionReason `json:"reason"`
	CreatedAt string         `json:"createdAt"`
}

type publishedNoteResponse struct {
	ID          string  `json:"id"`
	SpaceID     string  `json:"spaceId"`
	DirectoryID *string `json:"directoryId"`
	Title       string  `json:"title"`
	Slug        string  `json:"slug"`
	Markdown    string  `json:"markdown"`
	PublishedAt string  `json:"publishedAt"`
}

type trashEntryResponse struct {
	Note      noteResponse `json:"note"`
	TrashedAt string       `json:"trashedAt"`
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

func (handler *HTTPHandler) listNotes(response http.ResponseWriter, request *http.Request) {
	page, pageSize, ok := notePagination(response, request)
	if !ok {
		return
	}
	pageResult, err := handler.service.ListNoteSummaries(request.Context(), NoteListFilter{
		SpaceID:     request.URL.Query().Get("spaceId"),
		DirectoryID: request.URL.Query().Get("directoryId"),
		Page:        page,
		PageSize:    pageSize,
	})
	if writeNoteServiceError(response, err, "list") {
		return
	}
	data := make([]noteSummaryResponse, 0, len(pageResult.Notes))
	for _, note := range pageResult.Notes {
		data = append(data, toNoteSummaryResponse(note))
	}
	writeNotePage(response, data, pageResult.Page, pageResult.PageSize, pageResult.TotalItems)
}

func (handler *HTTPHandler) getPublishedNote(response http.ResponseWriter, request *http.Request) {
	note, err := handler.service.GetPublishedNote(request.Context(), request.PathValue("noteId"))
	if writeNoteServiceError(response, err, "load published") {
		return
	}
	_ = httpapi.WriteJSON(response, http.StatusOK, toPublishedNoteResponse(note))
}

func (handler *HTTPHandler) listPublishedNotes(response http.ResponseWriter, request *http.Request) {
	page, pageSize, ok := notePagination(response, request)
	if !ok {
		return
	}
	pageResult, err := handler.service.ListPublishedNotes(request.Context(), request.PathValue("spaceId"), page, pageSize)
	if writeNoteServiceError(response, err, "list published") {
		return
	}
	data := make([]publishedNoteResponse, 0, len(pageResult.Notes))
	for _, note := range pageResult.Notes {
		data = append(data, toPublishedNoteResponse(note))
	}
	writeNotePage(response, data, pageResult.Page, pageResult.PageSize, pageResult.TotalItems)
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

func (handler *HTTPHandler) moveNote(response http.ResponseWriter, request *http.Request) {
	var input struct {
		ExpectedVersion int64   `json:"expectedVersion"`
		SpaceID         string  `json:"spaceId"`
		DirectoryID     *string `json:"directoryId"`
		ConfirmPublish  bool    `json:"confirmPublish"`
	}
	if err := httpapi.DecodeJSON(request.Body, maxNoteRequestBytes, &input); err != nil {
		writeNoteDecodeError(response, err)
		return
	}
	directoryID := ""
	if input.DirectoryID != nil {
		directoryID = *input.DirectoryID
	}
	note, err := handler.service.Move(request.Context(), request.PathValue("noteId"), input.SpaceID, directoryID, input.ExpectedVersion, input.ConfirmPublish)
	if writeNoteServiceError(response, err, "move") {
		return
	}
	_ = httpapi.WriteJSON(response, http.StatusOK, toNoteResponse(note))
}

func (handler *HTTPHandler) createCheckpoint(response http.ResponseWriter, request *http.Request) {
	var input struct {
		ExpectedVersion int64 `json:"expectedVersion"`
	}
	if err := httpapi.DecodeJSON(request.Body, maxNoteRequestBytes, &input); err != nil {
		writeNoteDecodeError(response, err)
		return
	}
	revision, err := handler.service.CreateCheckpoint(request.Context(), request.PathValue("noteId"), input.ExpectedVersion)
	if writeNoteServiceError(response, err, "create Note Revision for") {
		return
	}
	response.Header().Set("Location", "/api/v1/notes/"+revision.NoteID+"/revisions/"+revision.ID)
	_ = httpapi.WriteJSON(response, http.StatusCreated, toRevisionResponse(revision))
}

func (handler *HTTPHandler) listRevisions(response http.ResponseWriter, request *http.Request) {
	page, err := revisionPaginationValue(request, "page", 1, 1, 1_000_000)
	if err != nil {
		httpapi.WriteError(response, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "page must be a positive integer")
		return
	}
	pageSize, err := revisionPaginationValue(request, "pageSize", 50, 1, 100)
	if err != nil {
		httpapi.WriteError(response, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "pageSize must be between 1 and 100")
		return
	}
	pageResult, err := handler.service.ListRevisions(request.Context(), request.PathValue("noteId"), page, pageSize)
	if writeNoteServiceError(response, err, "list revisions for") {
		return
	}
	data := make([]revisionResponse, 0, len(pageResult.Revisions))
	for _, revision := range pageResult.Revisions {
		data = append(data, toRevisionResponse(revision))
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

func (handler *HTTPHandler) restoreRevision(response http.ResponseWriter, request *http.Request) {
	var input struct {
		ExpectedVersion int64 `json:"expectedVersion"`
	}
	if err := httpapi.DecodeJSON(request.Body, maxNoteRequestBytes, &input); err != nil {
		writeNoteDecodeError(response, err)
		return
	}
	note, err := handler.service.Restore(request.Context(), request.PathValue("noteId"), request.PathValue("revisionId"), input.ExpectedVersion)
	if writeNoteServiceError(response, err, "restore revision for") {
		return
	}
	_ = httpapi.WriteJSON(response, http.StatusOK, toNoteResponse(note))
}

func (handler *HTTPHandler) trashNote(response http.ResponseWriter, request *http.Request) {
	var input struct {
		ExpectedVersion int64 `json:"expectedVersion"`
	}
	if err := httpapi.DecodeJSON(request.Body, maxNoteRequestBytes, &input); err != nil {
		writeNoteDecodeError(response, err)
		return
	}
	if err := handler.service.TrashNote(request.Context(), request.PathValue("noteId"), input.ExpectedVersion); writeNoteServiceError(response, err, "trash") {
		return
	}
	response.WriteHeader(http.StatusNoContent)
}

func (handler *HTTPHandler) listTrash(response http.ResponseWriter, request *http.Request) {
	page, pageSize, ok := notePagination(response, request)
	if !ok {
		return
	}
	pageResult, err := handler.service.ListTrash(request.Context(), page, pageSize)
	if writeNoteServiceError(response, err, "list Trash") {
		return
	}
	data := make([]trashEntryResponse, 0, len(pageResult.Entries))
	for _, entry := range pageResult.Entries {
		data = append(data, trashEntryResponse{
			Note:      toNoteResponse(entry.Note),
			TrashedAt: entry.TrashedAt.UTC().Format("2006-01-02T15:04:05.000000000Z07:00"),
		})
	}
	writeNotePage(response, data, pageResult.Page, pageResult.PageSize, pageResult.TotalItems)
}

func (handler *HTTPHandler) restoreFromTrash(response http.ResponseWriter, request *http.Request) {
	var input struct {
		SpaceID        string  `json:"spaceId"`
		DirectoryID    *string `json:"directoryId"`
		ConfirmPublish bool    `json:"confirmPublish"`
	}
	if err := httpapi.DecodeJSON(request.Body, maxNoteRequestBytes, &input); err != nil {
		writeNoteDecodeError(response, err)
		return
	}
	directoryID := ""
	if input.DirectoryID != nil {
		directoryID = *input.DirectoryID
	}
	note, err := handler.service.RestoreFromTrash(request.Context(), request.PathValue("noteId"), RestoreTrashInput{
		SpaceID:           input.SpaceID,
		DirectoryID:       directoryID,
		LocationSpecified: input.SpaceID != "" || input.DirectoryID != nil,
		ConfirmPublish:    input.ConfirmPublish,
	})
	if writeNoteServiceError(response, err, "restore from Trash") {
		return
	}
	_ = httpapi.WriteJSON(response, http.StatusOK, toNoteResponse(note))
}

func (handler *HTTPHandler) deleteTrashedNote(response http.ResponseWriter, request *http.Request) {
	if err := handler.service.DeleteTrashedNote(request.Context(), request.PathValue("noteId")); writeNoteServiceError(response, err, "permanently delete") {
		return
	}
	response.WriteHeader(http.StatusNoContent)
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
		UpdatedAt:   note.UpdatedAt().UTC().Format("2006-01-02T15:04:05.000000000Z07:00"),
	}
}

func toNoteSummaryResponse(note NoteSummary) noteSummaryResponse {
	var directoryID *string
	if note.DirectoryID != "" {
		value := note.DirectoryID
		directoryID = &value
	}
	return noteSummaryResponse{
		ID:          note.ID,
		SpaceID:     note.SpaceID,
		DirectoryID: directoryID,
		Title:       note.Title,
		Slug:        note.Slug,
		Version:     note.Version,
		IsPublished: note.IsPublished,
		UpdatedAt:   note.UpdatedAt.UTC().Format("2006-01-02T15:04:05.000000000Z07:00"),
	}
}

func toRevisionResponse(revision Revision) revisionResponse {
	return revisionResponse{
		ID:        revision.ID,
		NoteID:    revision.NoteID,
		Title:     revision.Title,
		Slug:      revision.Slug,
		Markdown:  revision.Markdown,
		Reason:    revision.Reason,
		CreatedAt: revision.CreatedAt.UTC().Format("2006-01-02T15:04:05.000000000Z07:00"),
	}
}

func toPublishedNoteResponse(note PublishedNote) publishedNoteResponse {
	var directoryID *string
	if note.DirectoryID != "" {
		value := note.DirectoryID
		directoryID = &value
	}
	return publishedNoteResponse{
		ID:          note.ID,
		SpaceID:     note.SpaceID,
		DirectoryID: directoryID,
		Title:       note.Title,
		Slug:        note.Slug,
		Markdown:    note.Markdown,
		PublishedAt: note.PublishedAt.Time.UTC().Format("2006-01-02T15:04:05.000000000Z07:00"),
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
	if errors.Is(err, ErrPublicRestoreConfirmationRequired) {
		httpapi.WriteError(response, http.StatusConflict, "PUBLIC_RESTORE_CONFIRMATION_REQUIRED", err.Error())
		return true
	}
	if errors.Is(err, ErrPublicMoveConfirmationRequired) {
		httpapi.WriteError(response, http.StatusConflict, "PUBLIC_MOVE_CONFIRMATION_REQUIRED", err.Error())
		return true
	}
	if errors.Is(err, ErrNoteNotFound) {
		httpapi.WriteError(response, http.StatusNotFound, "NOTE_NOT_FOUND", "Learning Note not found")
		return true
	}
	if errors.Is(err, ErrPublishedNoteNotFound) {
		httpapi.WriteError(response, http.StatusNotFound, "PUBLISHED_NOTE_NOT_FOUND", "published Learning Note not found")
		return true
	}
	if errors.Is(err, ErrTrashedNoteNotFound) {
		httpapi.WriteError(response, http.StatusNotFound, "TRASHED_NOTE_NOT_FOUND", "trashed Learning Note not found")
		return true
	}
	if errors.Is(err, ErrRevisionNotFound) {
		httpapi.WriteError(response, http.StatusNotFound, "REVISION_NOT_FOUND", "Note Revision not found")
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

func revisionPaginationValue(request *http.Request, key string, fallback, minimum, maximum int) (int, error) {
	raw := request.URL.Query().Get(key)
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < minimum || value > maximum {
		return 0, errors.New("pagination value is outside the allowed range")
	}
	return value, nil
}

func notePagination(response http.ResponseWriter, request *http.Request) (int, int, bool) {
	page, err := revisionPaginationValue(request, "page", 1, 1, 1_000_000)
	if err != nil {
		httpapi.WriteError(response, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "page must be a positive integer")
		return 0, 0, false
	}
	pageSize, err := revisionPaginationValue(request, "pageSize", 50, 1, 100)
	if err != nil {
		httpapi.WriteError(response, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "pageSize must be between 1 and 100")
		return 0, 0, false
	}
	return page, pageSize, true
}

func writeNotePage(response http.ResponseWriter, data any, page, pageSize, totalItems int) {
	totalPages := 0
	if totalItems > 0 {
		totalPages = (totalItems + pageSize - 1) / pageSize
	}
	_ = httpapi.WriteJSON(response, http.StatusOK, map[string]any{
		"data": data,
		"pagination": map[string]int{
			"page":       page,
			"pageSize":   pageSize,
			"totalItems": totalItems,
			"totalPages": totalPages,
		},
	})
}

func writeNoteDecodeError(response http.ResponseWriter, err error) {
	if errors.Is(err, httpapi.ErrBodyTooLarge) {
		httpapi.WriteError(response, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "request body is too large")
		return
	}
	httpapi.WriteError(response, http.StatusBadRequest, "INVALID_REQUEST", "request body must contain one valid JSON object")
}
