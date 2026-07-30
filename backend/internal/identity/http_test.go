package identity

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPHandlerStartsGitHubLoginWithProtectedStateCookie(t *testing.T) {
	service := NewService(ServiceConfig{
		KnowledgeOwnerGitHubID: 42,
		Store:                  &storeStub{},
		Provider:               &providerStub{},
		GenerateToken:          tokenSequence("oauth-state"),
		Now:                    time.Now,
	})
	handler := NewHTTPHandler(service, HTTPConfig{SecureCookies: true, PostLoginPath: "/workspace"})
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	request := httptest.NewRequest(http.MethodGet, "/auth/github/start", nil)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusFound {
		t.Fatalf("status code = %d, want %d", response.Code, http.StatusFound)
	}
	if got := response.Header().Get("Location"); got != "https://github.test/authorize?state=oauth-state" {
		t.Errorf("Location = %q", got)
	}
	cookie := response.Result().Cookies()[0]
	if cookie.Name != OAuthStateCookieName || cookie.Value != "oauth-state" {
		t.Errorf("OAuth cookie = %+v", cookie)
	}
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("OAuth cookie security flags = %+v", cookie)
	}
	if cookie.Path != "/auth/github/callback" {
		t.Errorf("OAuth cookie path = %q", cookie.Path)
	}
}

func TestHTTPHandlerCompletesLoginAndSetsOwnerSession(t *testing.T) {
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	service := NewService(ServiceConfig{
		KnowledgeOwnerGitHubID: 42,
		Store:                  &storeStub{consumeState: true},
		Provider: &providerStub{
			accessToken: "access-token",
			user:        GitHubUser{ID: 42, Login: "knowledge-owner"},
		},
		GenerateToken: tokenSequence("session-token"),
		Now:           func() time.Time { return now },
	})
	handler := NewHTTPHandler(service, HTTPConfig{SecureCookies: true, PostLoginPath: "/workspace"})
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	request := httptest.NewRequest(http.MethodGet, "/auth/github/callback?state=oauth-state&code=github-code", nil)
	request.AddCookie(&http.Cookie{Name: OAuthStateCookieName, Value: "oauth-state"})
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusFound {
		t.Fatalf("status code = %d, want %d", response.Code, http.StatusFound)
	}
	if got := response.Header().Get("Location"); got != "/workspace" {
		t.Errorf("Location = %q", got)
	}
	var sessionCookie *http.Cookie
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == SessionCookieName {
			sessionCookie = cookie
		}
	}
	if sessionCookie == nil {
		t.Fatal("owner session cookie was not set")
	}
	if sessionCookie.Value != "session-token" || !sessionCookie.HttpOnly || !sessionCookie.Secure {
		t.Errorf("session cookie = %+v", sessionCookie)
	}
	if sessionCookie.SameSite != http.SameSiteLaxMode || sessionCookie.Path != "/" {
		t.Errorf("session cookie scope = %+v", sessionCookie)
	}
}

func TestHTTPHandlerRejectsNonOwnerWithoutLeakingDetails(t *testing.T) {
	service := NewService(ServiceConfig{
		KnowledgeOwnerGitHubID: 42,
		Store:                  &storeStub{consumeState: true},
		Provider: &providerStub{
			accessToken: "access-token",
			user:        GitHubUser{ID: 99, Login: "someone-else"},
		},
		GenerateToken: tokenSequence("unused"),
		Now:           time.Now,
	})
	handler := NewHTTPHandler(service, HTTPConfig{})
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	request := httptest.NewRequest(http.MethodGet, "/auth/github/callback?state=oauth-state&code=github-code", nil)
	request.AddCookie(&http.Cookie{Name: OAuthStateCookieName, Value: "oauth-state"})
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status code = %d, want %d", response.Code, http.StatusForbidden)
	}
	if got := response.Body.String(); got != `{"error":{"code":"FORBIDDEN","message":"GitHub account is not authorized"}}`+"\n" {
		t.Errorf("body = %q", got)
	}
}

func TestHTTPHandlerReturnsCurrentOwnerSession(t *testing.T) {
	store := &storeStub{foundSession: Session{
		Owner:     Owner{GitHubID: 42, Login: "knowledge-owner", AvatarURL: "https://avatars.test/owner"},
		ExpiresAt: time.Now().Add(time.Hour),
	}}
	service := NewService(ServiceConfig{
		KnowledgeOwnerGitHubID: 42,
		Store:                  store,
		Provider:               &providerStub{},
		GenerateToken:          tokenSequence("unused"),
		Now:                    time.Now,
	})
	handler := NewHTTPHandler(service, HTTPConfig{})
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/session", nil)
	request.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "session-token"})
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", response.Code, http.StatusOK)
	}
	var body struct {
		Owner struct {
			GitHubID int64  `json:"githubUserId"`
			Login    string `json:"login"`
		} `json:"owner"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Owner.GitHubID != 42 || body.Owner.Login != "knowledge-owner" {
		t.Errorf("owner = %+v", body.Owner)
	}
}

func TestRequireOwnerRejectsMissingSession(t *testing.T) {
	service := NewService(ServiceConfig{
		KnowledgeOwnerGitHubID: 42,
		Store:                  &storeStub{},
		Provider:               &providerStub{},
		GenerateToken:          tokenSequence("unused"),
		Now:                    time.Now,
	})
	handler := NewHTTPHandler(service, HTTPConfig{})
	protected := handler.RequireOwner(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("protected handler was called")
	}))

	response := httptest.NewRecorder()
	protected.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/protected", nil))

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status code = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestRequireOwnerAddsSessionToRequestContext(t *testing.T) {
	wantSession := Session{Owner: Owner{GitHubID: 42, Login: "knowledge-owner"}}
	service := NewService(ServiceConfig{
		KnowledgeOwnerGitHubID: 42,
		Store:                  &storeStub{foundSession: wantSession},
		Provider:               &providerStub{},
		GenerateToken:          tokenSequence("unused"),
		Now:                    time.Now,
	})
	handler := NewHTTPHandler(service, HTTPConfig{})
	protected := handler.RequireOwner(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if got, ok := SessionFromContext(request.Context()); !ok || got != wantSession {
			t.Errorf("session = %+v, ok = %v", got, ok)
		}
		response.WriteHeader(http.StatusNoContent)
	}))

	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	request.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "session-token"})
	response := httptest.NewRecorder()
	protected.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status code = %d, want %d", response.Code, http.StatusNoContent)
	}
}
