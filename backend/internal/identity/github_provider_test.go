package identity

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestGitHubProviderBuildsAuthorizationURL(t *testing.T) {
	provider := NewGitHubProvider(GitHubProviderConfig{
		ClientID:              "client-id",
		ClientSecret:          "client-secret",
		RedirectURL:           "https://notes.test/auth/github/callback",
		AuthorizationEndpoint: "https://github.test/login/oauth/authorize",
	})

	authorizationURL, err := url.Parse(provider.AuthorizationURL("oauth-state"))
	if err != nil {
		t.Fatalf("parse authorization URL: %v", err)
	}
	query := authorizationURL.Query()
	if got := query.Get("client_id"); got != "client-id" {
		t.Errorf("client_id = %q", got)
	}
	if got := query.Get("redirect_uri"); got != "https://notes.test/auth/github/callback" {
		t.Errorf("redirect_uri = %q", got)
	}
	if got := query.Get("state"); got != "oauth-state" {
		t.Errorf("state = %q", got)
	}
	if got := query.Get("scope"); got != "read:user" {
		t.Errorf("scope = %q", got)
	}
}

func TestGitHubProviderExchangesCodeAndLoadsUser(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("POST /login/oauth/access_token", func(response http.ResponseWriter, request *http.Request) {
		if err := request.ParseForm(); err != nil {
			t.Fatalf("ParseForm() error = %v", err)
		}
		if got := request.Form.Get("client_id"); got != "client-id" {
			t.Errorf("client_id = %q", got)
		}
		if got := request.Form.Get("client_secret"); got != "client-secret" {
			t.Errorf("client_secret = %q", got)
		}
		if got := request.Form.Get("code"); got != "github-code" {
			t.Errorf("code = %q", got)
		}
		if got := request.Header.Get("Accept"); got != "application/json" {
			t.Errorf("Accept = %q", got)
		}
		_ = json.NewEncoder(response).Encode(map[string]string{"access_token": "access-token"})
	})
	mux.HandleFunc("GET /user", func(response http.ResponseWriter, request *http.Request) {
		if got := request.Header.Get("Authorization"); got != "Bearer access-token" {
			t.Errorf("Authorization = %q", got)
		}
		if got := request.Header.Get("X-GitHub-Api-Version"); got != "2022-11-28" {
			t.Errorf("X-GitHub-Api-Version = %q", got)
		}
		_ = json.NewEncoder(response).Encode(map[string]any{
			"id":         42,
			"login":      "knowledge-owner",
			"avatar_url": "https://avatars.test/owner",
		})
	})

	provider := NewGitHubProvider(GitHubProviderConfig{
		ClientID:      "client-id",
		ClientSecret:  "client-secret",
		RedirectURL:   "https://notes.test/auth/github/callback",
		HTTPClient:    server.Client(),
		TokenEndpoint: server.URL + "/login/oauth/access_token",
		UserEndpoint:  server.URL + "/user",
	})

	accessToken, err := provider.ExchangeCode(context.Background(), "github-code")
	if err != nil {
		t.Fatalf("ExchangeCode() error = %v", err)
	}
	user, err := provider.AuthenticatedUser(context.Background(), accessToken)
	if err != nil {
		t.Fatalf("AuthenticatedUser() error = %v", err)
	}
	if user.ID != 42 || user.Login != "knowledge-owner" || user.AvatarURL != "https://avatars.test/owner" {
		t.Errorf("user = %+v", user)
	}
}

func TestGitHubProviderRejectsTokenResponseWithoutAccessToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(response).Encode(map[string]string{"error": "bad_verification_code"})
	}))
	defer server.Close()

	provider := NewGitHubProvider(GitHubProviderConfig{
		ClientID:      "client-id",
		ClientSecret:  "client-secret",
		RedirectURL:   "https://notes.test/auth/github/callback",
		HTTPClient:    server.Client(),
		TokenEndpoint: server.URL,
	})

	if _, err := provider.ExchangeCode(context.Background(), "bad-code"); err == nil {
		t.Fatal("ExchangeCode() error = nil, want error")
	}
}
