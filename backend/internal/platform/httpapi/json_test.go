package httpapi

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeJSONAcceptsOneKnownObject(t *testing.T) {
	var input struct {
		Name string `json:"name"`
	}

	err := DecodeJSON(strings.NewReader(`{"name":"AI Agent"}`), 1024, &input)
	if err != nil {
		t.Fatalf("DecodeJSON() error = %v", err)
	}
	if input.Name != "AI Agent" {
		t.Errorf("Name = %q, want %q", input.Name, "AI Agent")
	}
}

func TestDecodeJSONRejectsUnknownFields(t *testing.T) {
	var input struct {
		Name string `json:"name"`
	}

	err := DecodeJSON(strings.NewReader(`{"name":"AI Agent","owner":"other"}`), 1024, &input)
	if !errors.Is(err, ErrInvalidJSON) {
		t.Fatalf("DecodeJSON() error = %v, want %v", err, ErrInvalidJSON)
	}
}

func TestDecodeJSONRejectsMultipleValues(t *testing.T) {
	var input map[string]any

	err := DecodeJSON(strings.NewReader(`{} {}`), 1024, &input)
	if !errors.Is(err, ErrInvalidJSON) {
		t.Fatalf("DecodeJSON() error = %v, want %v", err, ErrInvalidJSON)
	}
}

func TestDecodeJSONRejectsOversizedBodies(t *testing.T) {
	var input map[string]any

	err := DecodeJSON(strings.NewReader(`{"name":"AI Agent"}`), 8, &input)
	if !errors.Is(err, ErrBodyTooLarge) {
		t.Fatalf("DecodeJSON() error = %v, want %v", err, ErrBodyTooLarge)
	}
}

func TestWriteErrorUsesSharedEnvelope(t *testing.T) {
	response := httptest.NewRecorder()

	WriteError(response, 422, "VALIDATION_ERROR", "invalid request")

	if response.Code != 422 {
		t.Fatalf("status code = %d, want 422", response.Code)
	}
	if got := response.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	if got := response.Body.String(); got != `{"error":{"code":"VALIDATION_ERROR","message":"invalid request"}}`+"\n" {
		t.Errorf("body = %q", got)
	}
}
