package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSecurityHeadersProtectAPIResponses(t *testing.T) {
	handler := SecurityHeaders(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	}), true)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))

	wantHeaders := map[string]string{
		"Content-Security-Policy":   "default-src 'none'; frame-ancestors 'none'",
		"Strict-Transport-Security": "max-age=31536000; includeSubDomains",
		"X-Content-Type-Options":    "nosniff",
		"X-Frame-Options":           "DENY",
		"Referrer-Policy":           "strict-origin-when-cross-origin",
	}
	for name, want := range wantHeaders {
		if got := response.Header().Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}

func TestFixedWindowLimiterRejectsRequestsBeyondLimit(t *testing.T) {
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	limiter := NewFixedWindowLimiter(FixedWindowConfig{
		Limit:  1,
		Window: time.Minute,
		Now:    func() time.Time { return now },
	})
	handler := limiter.Middleware(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	}))

	firstRequest := httptest.NewRequest(http.MethodGet, "/auth/github/start", nil)
	firstRequest.RemoteAddr = "192.0.2.10:1234"
	firstResponse := httptest.NewRecorder()
	handler.ServeHTTP(firstResponse, firstRequest)
	if firstResponse.Code != http.StatusNoContent {
		t.Fatalf("first status = %d, want %d", firstResponse.Code, http.StatusNoContent)
	}

	secondRequest := httptest.NewRequest(http.MethodGet, "/auth/github/start", nil)
	secondRequest.RemoteAddr = "192.0.2.10:5678"
	secondResponse := httptest.NewRecorder()
	handler.ServeHTTP(secondResponse, secondRequest)
	if secondResponse.Code != http.StatusTooManyRequests {
		t.Fatalf("second status = %d, want %d", secondResponse.Code, http.StatusTooManyRequests)
	}
	if got := secondResponse.Header().Get("Retry-After"); got != "60" {
		t.Errorf("Retry-After = %q, want %q", got, "60")
	}
}

func TestFixedWindowLimiterUsesForwardedClientAddress(t *testing.T) {
	limiter := NewFixedWindowLimiter(FixedWindowConfig{Limit: 1, Window: time.Minute, TrustForwardedFor: true})
	handler := limiter.Middleware(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	}))

	for _, address := range []string{"192.0.2.10", "192.0.2.11"} {
		request := httptest.NewRequest(http.MethodGet, "/auth/github/start", nil)
		request.RemoteAddr = "172.20.0.5:8080"
		request.Header.Set("X-Forwarded-For", address)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusNoContent {
			t.Errorf("client %s status = %d, want %d", address, response.Code, http.StatusNoContent)
		}
	}
}
