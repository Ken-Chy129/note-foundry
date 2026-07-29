package attachments

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHTTPHandlerUploadsListsAndDownloadsAttachment(t *testing.T) {
	repository := &attachmentRepositoryStub{noteExists: true}
	storage := &storageStub{}
	ids := []string{"11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222"}
	index := 0
	service := NewService(repository, storage, func() string {
		id := ids[index]
		index++
		return id
	}, func() time.Time { return time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC) })
	handler := NewHTTPHandler(service, allowAttachmentRequest)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "diagram.svg")
	if err != nil {
		t.Fatalf("CreateFormFile() error = %v", err)
	}
	_, _ = part.Write([]byte("<svg></svg>"))
	_ = writer.Close()
	uploadRequest := httptest.NewRequest(http.MethodPost, "/api/v1/notes/33333333-3333-4333-8333-333333333333/attachments", &body)
	uploadRequest.Header.Set("Content-Type", writer.FormDataContentType())
	uploadResponse := httptest.NewRecorder()
	mux.ServeHTTP(uploadResponse, uploadRequest)
	if uploadResponse.Code != http.StatusCreated || !strings.Contains(uploadResponse.Body.String(), "diagram.svg") {
		t.Fatalf("upload response = %d %s", uploadResponse.Code, uploadResponse.Body.String())
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/notes/33333333-3333-4333-8333-333333333333/attachments", nil)
	listResponse := httptest.NewRecorder()
	mux.ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK || !strings.Contains(listResponse.Body.String(), ids[0]) {
		t.Fatalf("list response = %d %s", listResponse.Code, listResponse.Body.String())
	}

	ownerRequest := httptest.NewRequest(http.MethodGet, "/api/v1/attachments/11111111-1111-4111-8111-111111111111", nil)
	ownerResponse := httptest.NewRecorder()
	mux.ServeHTTP(ownerResponse, ownerRequest)
	if ownerResponse.Code != http.StatusOK || ownerResponse.Body.String() != "<svg></svg>" || ownerResponse.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("owner download = %d %q headers=%v", ownerResponse.Code, ownerResponse.Body.String(), ownerResponse.Header())
	}
	if !strings.HasPrefix(ownerResponse.Header().Get("Content-Disposition"), "attachment;") {
		t.Errorf("SVG Content-Disposition = %q, want forced download", ownerResponse.Header().Get("Content-Disposition"))
	}
}

func allowAttachmentRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		next.ServeHTTP(response, request.WithContext(context.Background()))
	})
}
