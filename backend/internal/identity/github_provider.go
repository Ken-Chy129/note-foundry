package identity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const githubResponseLimit = 1 << 20

type GitHubProviderConfig struct {
	ClientID              string
	ClientSecret          string
	RedirectURL           string
	HTTPClient            *http.Client
	AuthorizationEndpoint string
	TokenEndpoint         string
	UserEndpoint          string
}

type githubProvider struct {
	clientID              string
	clientSecret          string
	redirectURL           string
	httpClient            *http.Client
	authorizationEndpoint string
	tokenEndpoint         string
	userEndpoint          string
}

func NewGitHubProvider(config GitHubProviderConfig) GitHubProvider {
	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}

	return &githubProvider{
		clientID:              config.ClientID,
		clientSecret:          config.ClientSecret,
		redirectURL:           config.RedirectURL,
		httpClient:            httpClient,
		authorizationEndpoint: stringOrDefault(config.AuthorizationEndpoint, "https://github.com/login/oauth/authorize"),
		tokenEndpoint:         stringOrDefault(config.TokenEndpoint, "https://github.com/login/oauth/access_token"),
		userEndpoint:          stringOrDefault(config.UserEndpoint, "https://api.github.com/user"),
	}
}

func (provider *githubProvider) AuthorizationURL(state string) string {
	query := url.Values{
		"client_id":    {provider.clientID},
		"redirect_uri": {provider.redirectURL},
		"scope":        {"read:user"},
		"state":        {state},
	}
	return provider.authorizationEndpoint + "?" + query.Encode()
}

func (provider *githubProvider) ExchangeCode(ctx context.Context, code string) (string, error) {
	form := url.Values{
		"client_id":     {provider.clientID},
		"client_secret": {provider.clientSecret},
		"code":          {code},
		"redirect_uri":  {provider.redirectURL},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, provider.tokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("create GitHub token request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("User-Agent", "NoteFoundry/0.1")

	response, err := provider.httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("send GitHub token request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("GitHub token endpoint returned status %d", response.StatusCode)
	}

	var payload struct {
		AccessToken      string `json:"access_token"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if err := decodeLimitedJSON(response.Body, &payload); err != nil {
		return "", fmt.Errorf("decode GitHub token response: %w", err)
	}
	if payload.AccessToken == "" {
		if payload.Error != "" {
			return "", fmt.Errorf("GitHub token exchange failed: %s", payload.Error)
		}
		return "", errors.New("GitHub token response did not include an access token")
	}
	return payload.AccessToken, nil
}

func (provider *githubProvider) AuthenticatedUser(ctx context.Context, accessToken string) (GitHubUser, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, provider.userEndpoint, nil)
	if err != nil {
		return GitHubUser{}, fmt.Errorf("create GitHub user request: %w", err)
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("Authorization", "Bearer "+accessToken)
	request.Header.Set("User-Agent", "NoteFoundry/0.1")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	response, err := provider.httpClient.Do(request)
	if err != nil {
		return GitHubUser{}, fmt.Errorf("send GitHub user request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return GitHubUser{}, fmt.Errorf("GitHub user endpoint returned status %d", response.StatusCode)
	}

	var payload struct {
		ID        int64  `json:"id"`
		Login     string `json:"login"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := decodeLimitedJSON(response.Body, &payload); err != nil {
		return GitHubUser{}, fmt.Errorf("decode GitHub user response: %w", err)
	}
	if payload.ID <= 0 || strings.TrimSpace(payload.Login) == "" {
		return GitHubUser{}, errors.New("GitHub user response is missing required identity fields")
	}
	return GitHubUser{ID: payload.ID, Login: payload.Login, AvatarURL: payload.AvatarURL}, nil
}

func decodeLimitedJSON(reader io.Reader, destination any) error {
	decoder := json.NewDecoder(io.LimitReader(reader, githubResponseLimit))
	return decoder.Decode(destination)
}

func stringOrDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
