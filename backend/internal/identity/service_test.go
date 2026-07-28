package identity

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"
)

func TestStartLoginStoresHashedSingleUseState(t *testing.T) {
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	store := &storeStub{}
	provider := &providerStub{}
	service := NewService(ServiceConfig{
		KnowledgeOwnerGitHubID: 42,
		Store:                  store,
		Provider:               provider,
		GenerateToken:          tokenSequence("raw-oauth-state"),
		Now:                    func() time.Time { return now },
	})

	start, err := service.StartLogin(context.Background())
	if err != nil {
		t.Fatalf("StartLogin() error = %v", err)
	}

	if start.State != "raw-oauth-state" {
		t.Errorf("State = %q", start.State)
	}
	if start.AuthorizationURL != "https://github.test/authorize?state=raw-oauth-state" {
		t.Errorf("AuthorizationURL = %q", start.AuthorizationURL)
	}
	if store.savedStateDigest != sha256.Sum256([]byte("raw-oauth-state")) {
		t.Error("stored OAuth state digest does not match the raw state")
	}
	if store.savedStateExpiresAt != now.Add(10*time.Minute) {
		t.Errorf("state expiry = %v", store.savedStateExpiresAt)
	}
}

func TestCompleteLoginRejectsMismatchedBrowserState(t *testing.T) {
	service := NewService(ServiceConfig{
		KnowledgeOwnerGitHubID: 42,
		Store:                  &storeStub{consumeState: true},
		Provider:               &providerStub{},
		GenerateToken:          tokenSequence("session-token"),
		Now:                    time.Now,
	})

	_, err := service.CompleteLogin(context.Background(), "cookie-state", "query-state", "code")
	if !errors.Is(err, ErrInvalidOAuthState) {
		t.Fatalf("CompleteLogin() error = %v, want %v", err, ErrInvalidOAuthState)
	}
}

func TestCompleteLoginCreatesOwnerSession(t *testing.T) {
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	store := &storeStub{consumeState: true}
	provider := &providerStub{
		accessToken: "github-access-token",
		user: GitHubUser{
			ID:        42,
			Login:     "knowledge-owner",
			AvatarURL: "https://avatars.test/owner",
		},
	}
	service := NewService(ServiceConfig{
		KnowledgeOwnerGitHubID: 42,
		Store:                  store,
		Provider:               provider,
		GenerateToken:          tokenSequence("session-token"),
		Now:                    func() time.Time { return now },
	})

	result, err := service.CompleteLogin(context.Background(), "oauth-state", "oauth-state", "github-code")
	if err != nil {
		t.Fatalf("CompleteLogin() error = %v", err)
	}

	if provider.exchangedCode != "github-code" {
		t.Errorf("exchanged code = %q", provider.exchangedCode)
	}
	if provider.userAccessToken != "github-access-token" {
		t.Errorf("user access token = %q", provider.userAccessToken)
	}
	if result.SessionToken != "session-token" {
		t.Errorf("SessionToken = %q", result.SessionToken)
	}
	if result.Session.Owner.GitHubID != 42 || result.Session.Owner.Login != "knowledge-owner" {
		t.Errorf("owner = %+v", result.Session.Owner)
	}
	if store.savedSessionDigest != sha256.Sum256([]byte("session-token")) {
		t.Error("stored session digest does not match the raw session token")
	}
	if store.savedSessionExpiresAt != now.Add(30*24*time.Hour) {
		t.Errorf("session expiry = %v", store.savedSessionExpiresAt)
	}
}

func TestCompleteLoginRejectsAnotherGitHubAccount(t *testing.T) {
	store := &storeStub{consumeState: true}
	service := NewService(ServiceConfig{
		KnowledgeOwnerGitHubID: 42,
		Store:                  store,
		Provider: &providerStub{
			accessToken: "github-access-token",
			user:        GitHubUser{ID: 99, Login: "someone-else"},
		},
		GenerateToken: tokenSequence("session-token"),
		Now:           time.Now,
	})

	_, err := service.CompleteLogin(context.Background(), "oauth-state", "oauth-state", "github-code")
	if !errors.Is(err, ErrNotKnowledgeOwner) {
		t.Fatalf("CompleteLogin() error = %v, want %v", err, ErrNotKnowledgeOwner)
	}
	if store.savedSession {
		t.Error("session was stored for a non-owner GitHub account")
	}
}

func TestAuthenticateUsesHashedSessionToken(t *testing.T) {
	want := Session{Owner: Owner{GitHubID: 42, Login: "knowledge-owner"}}
	store := &storeStub{foundSession: want}
	service := NewService(ServiceConfig{
		KnowledgeOwnerGitHubID: 42,
		Store:                  store,
		Provider:               &providerStub{},
		GenerateToken:          tokenSequence("unused"),
		Now:                    time.Now,
	})

	got, err := service.Authenticate(context.Background(), "raw-session-token")
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if got != want {
		t.Errorf("session = %+v, want %+v", got, want)
	}
	if store.foundSessionDigest != sha256.Sum256([]byte("raw-session-token")) {
		t.Error("session lookup did not hash the raw cookie token")
	}
}

type storeStub struct {
	savedStateDigest      [32]byte
	savedStateExpiresAt   time.Time
	consumeState          bool
	savedSession          bool
	savedSessionDigest    [32]byte
	savedSessionValue     Session
	savedSessionExpiresAt time.Time
	foundSessionDigest    [32]byte
	foundSession          Session
}

func (store *storeStub) SaveOAuthState(_ context.Context, digest [32]byte, expiresAt time.Time) error {
	store.savedStateDigest = digest
	store.savedStateExpiresAt = expiresAt
	return nil
}

func (store *storeStub) ConsumeOAuthState(_ context.Context, _ [32]byte, _ time.Time) (bool, error) {
	return store.consumeState, nil
}

func (store *storeStub) SaveSession(_ context.Context, digest [32]byte, session Session, expiresAt time.Time) error {
	store.savedSession = true
	store.savedSessionDigest = digest
	store.savedSessionValue = session
	store.savedSessionExpiresAt = expiresAt
	return nil
}

func (store *storeStub) FindSession(_ context.Context, digest [32]byte, _ time.Time) (Session, error) {
	store.foundSessionDigest = digest
	return store.foundSession, nil
}

func (store *storeStub) DeleteSession(context.Context, [32]byte) error {
	return nil
}

type providerStub struct {
	accessToken     string
	user            GitHubUser
	exchangedCode   string
	userAccessToken string
}

func (provider *providerStub) AuthorizationURL(state string) string {
	return "https://github.test/authorize?state=" + state
}

func (provider *providerStub) ExchangeCode(_ context.Context, code string) (string, error) {
	provider.exchangedCode = code
	return provider.accessToken, nil
}

func (provider *providerStub) AuthenticatedUser(_ context.Context, accessToken string) (GitHubUser, error) {
	provider.userAccessToken = accessToken
	return provider.user, nil
}

func tokenSequence(tokens ...string) TokenGenerator {
	index := 0
	return func() (string, error) {
		token := tokens[index]
		index++
		return token, nil
	}
}
