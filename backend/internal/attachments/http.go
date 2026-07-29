package attachments

import (
	"errors"
	"mime"
	"net/http"
	"strconv"
	"time"

	"github.com/Ken-Chy129/note-foundry/backend/internal/platform/httpapi"
)

const maxAttachmentBytes = 25 * 1024 * 1024

type OwnerMiddleware func(http.Handler) http.Handler

type HTTPHandler struct {
	service      *Service
	requireOwner OwnerMiddleware
}

func NewHTTPHandler(service *Service, requireOwner OwnerMiddleware) *HTTPHandler {
	return &HTTPHandler{service: service, requireOwner: requireOwner}
}

func (handler *HTTPHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle("POST /api/v1/notes/{noteId}/attachments", handler.requireOwner(http.HandlerFunc(handler.upload)))
	mux.Handle("GET /api/v1/notes/{noteId}/attachments", handler.requireOwner(http.HandlerFunc(handler.listForNote)))
	mux.Handle("GET /api/v1/attachments/{attachmentId}", handler.requireOwner(http.HandlerFunc(handler.downloadOwner)))
	mux.HandleFunc("GET /api/v1/public/attachments/{attachmentId}", handler.downloadPublic)
}

type attachmentResponse struct {
	ID           string  `json:"id"`
	NoteID       string  `json:"noteId"`
	OriginalName string  `json:"originalName"`
	MediaType    string  `json:"mediaType"`
	SizeBytes    int64   `json:"sizeBytes"`
	SHA256Hex    string  `json:"sha256"`
	PublishedAt  *string `json:"publishedAt"`
	CreatedAt    string  `json:"createdAt"`
}

func (handler *HTTPHandler) upload(response http.ResponseWriter, request *http.Request) {
	request.Body = http.MaxBytesReader(response, request.Body, maxAttachmentBytes+1024*1024)
	if err := request.ParseMultipartForm(maxAttachmentBytes); err != nil {
		httpapi.WriteError(response, http.StatusRequestEntityTooLarge, "ATTACHMENT_TOO_LARGE", "Attachment exceeds the 25 MiB limit")
		return
	}
	if request.MultipartForm != nil {
		defer request.MultipartForm.RemoveAll()
	}
	file, header, err := request.FormFile("file")
	if err != nil {
		httpapi.WriteError(response, http.StatusBadRequest, "INVALID_REQUEST", "multipart field file is required")
		return
	}
	defer file.Close()
	attachment, err := handler.service.Upload(request.Context(), request.PathValue("noteId"), header.Filename, file)
	if writeAttachmentError(response, err) {
		return
	}
	response.Header().Set("Location", "/api/v1/attachments/"+attachment.ID)
	_ = httpapi.WriteJSON(response, http.StatusCreated, toAttachmentResponse(attachment))
}

func (handler *HTTPHandler) listForNote(response http.ResponseWriter, request *http.Request) {
	attachments, err := handler.service.ListForNote(request.Context(), request.PathValue("noteId"))
	if writeAttachmentError(response, err) {
		return
	}
	data := make([]attachmentResponse, 0, len(attachments))
	for _, attachment := range attachments {
		data = append(data, toAttachmentResponse(attachment))
	}
	_ = httpapi.WriteJSON(response, http.StatusOK, map[string]any{"data": data})
}

func (handler *HTTPHandler) downloadOwner(response http.ResponseWriter, request *http.Request) {
	attachment, content, err := handler.service.OpenOwner(request.Context(), request.PathValue("attachmentId"))
	if writeAttachmentError(response, err) {
		return
	}
	defer content.Close()
	response.Header().Set("Cache-Control", "private, no-store")
	serveAttachment(response, request, attachment, content)
}

func (handler *HTTPHandler) downloadPublic(response http.ResponseWriter, request *http.Request) {
	attachment, content, err := handler.service.OpenPublic(request.Context(), request.PathValue("attachmentId"))
	if writeAttachmentError(response, err) {
		return
	}
	defer content.Close()
	response.Header().Set("Cache-Control", "public, max-age=300")
	serveAttachment(response, request, attachment, content)
}

func serveAttachment(response http.ResponseWriter, request *http.Request, attachment Attachment, content ReadSeekCloser) {
	response.Header().Set("Content-Type", attachment.MediaType)
	disposition := "attachment"
	if safeInlineMediaType(attachment.MediaType) {
		disposition = "inline"
	}
	response.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": attachment.OriginalName}))
	response.Header().Set("X-Content-Type-Options", "nosniff")
	response.Header().Set("Content-Length", strconv.FormatInt(attachment.SizeBytes, 10))
	http.ServeContent(response, request, attachment.OriginalName, attachment.CreatedAt, content)
}

func safeInlineMediaType(mediaType string) bool {
	switch mediaType {
	case "image/png", "image/jpeg", "image/gif", "image/webp", "image/avif":
		return true
	default:
		return false
	}
}

func toAttachmentResponse(attachment Attachment) attachmentResponse {
	var publishedAt *string
	if attachment.PublishedAt != nil {
		value := attachment.PublishedAt.UTC().Format(time.RFC3339Nano)
		publishedAt = &value
	}
	return attachmentResponse{
		ID:           attachment.ID,
		NoteID:       attachment.NoteID,
		OriginalName: attachment.OriginalName,
		MediaType:    attachment.MediaType,
		SizeBytes:    attachment.SizeBytes,
		SHA256Hex:    attachment.SHA256Hex,
		PublishedAt:  publishedAt,
		CreatedAt:    attachment.CreatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func writeAttachmentError(response http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrAttachmentNameRequired) || errors.Is(err, ErrAttachmentEmpty) {
		httpapi.WriteError(response, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error())
		return true
	}
	if errors.Is(err, ErrAttachmentNotFound) {
		httpapi.WriteError(response, http.StatusNotFound, "ATTACHMENT_NOT_FOUND", "Attachment not found")
		return true
	}
	if errors.Is(err, ErrAttachmentNoteNotFound) {
		httpapi.WriteError(response, http.StatusNotFound, "NOTE_NOT_FOUND", "Learning Note not found")
		return true
	}
	httpapi.WriteError(response, http.StatusInternalServerError, "INTERNAL_ERROR", "could not process Attachment")
	return true
}
