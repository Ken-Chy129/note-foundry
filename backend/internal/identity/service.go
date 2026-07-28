package identity

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"time"
)

var (
	ErrInvalidOAuthState         = errors.New("invalid or expired OAuth state")
	ErrAuthorizationCodeRequired = errors.New("GitHub authorization code is required")
	ErrNotKnowledgeOwner         = errors.New("GitHub account is not the configured Knowledge Owner")
	ErrUnauthenticated           = errors.New("owner session is missing or expired")
	ErrSessionNotFound           = errors.New("owner session not found")
)

const (
	oauthStateLifetime = 10 * time.Minute
	sessionLifetime    = 30 * 24 * time.Hour
)

type Owner struct {
	GitHubID  int64
	Login     string
	AvatarURL string
}

type Session struct {
	Owner     Owner
	ExpiresAt time.Time
}

type GitHubUser struct {
	ID        int64
	Login     string
	AvatarURL string
}

type LoginStart struct {
	State            string
	AuthorizationURL string
	ExpiresAt        time.Time
}

type LoginResult struct {
	SessionToken string
	Session      Session
}

type Store interface {
	SaveOAuthState(context.Context, [32]byte, time.Time) error
	ConsumeOAuthState(context.Context, [32]byte, time.Time) (bool, error)
	SaveSession(context.Context, [32]byte, Session, time.Time) error
	FindSession(context.Context, [32]byte, time.Time) (Session, error)
	DeleteSession(context.Context, [32]byte) error
}

type GitHubProvider interface {
	AuthorizationURL(state string) string
	ExchangeCode(context.Context, string) (string, error)
	AuthenticatedUser(context.Context, string) (GitHubUser, error)
}

type TokenGenerator func() (string, error)

type ServiceConfig struct {
	KnowledgeOwnerGitHubID int64
	Store                  Store
	Provider               GitHubProvider
	GenerateToken          TokenGenerator
	Now                    func() time.Time
}

type Service struct {
	knowledgeOwnerGitHubID int64
	store                  Store
	provider               GitHubProvider
	generateToken          TokenGenerator
	now                    func() time.Time
}

func NewService(config ServiceConfig) *Service {
	return &Service{
		knowledgeOwnerGitHubID: config.KnowledgeOwnerGitHubID,
		store:                  config.Store,
		provider:               config.Provider,
		generateToken:          config.GenerateToken,
		now:                    config.Now,
	}
}

func (service *Service) StartLogin(ctx context.Context) (LoginStart, error) {
	state, err := service.generateToken()
	if err != nil {
		return LoginStart{}, fmt.Errorf("generate OAuth state: %w", err)
	}

	expiresAt := service.now().Add(oauthStateLifetime)
	if err := service.store.SaveOAuthState(ctx, tokenDigest(state), expiresAt); err != nil {
		return LoginStart{}, fmt.Errorf("save OAuth state: %w", err)
	}

	return LoginStart{
		State:            state,
		AuthorizationURL: service.provider.AuthorizationURL(state),
		ExpiresAt:        expiresAt,
	}, nil
}

func (service *Service) CompleteLogin(ctx context.Context, browserState, callbackState, code string) (LoginResult, error) {
	if browserState == "" || callbackState == "" || subtle.ConstantTimeCompare([]byte(browserState), []byte(callbackState)) != 1 {
		return LoginResult{}, ErrInvalidOAuthState
	}
	if code == "" {
		return LoginResult{}, ErrAuthorizationCodeRequired
	}

	valid, err := service.store.ConsumeOAuthState(ctx, tokenDigest(callbackState), service.now())
	if err != nil {
		return LoginResult{}, fmt.Errorf("consume OAuth state: %w", err)
	}
	if !valid {
		return LoginResult{}, ErrInvalidOAuthState
	}

	accessToken, err := service.provider.ExchangeCode(ctx, code)
	if err != nil {
		return LoginResult{}, fmt.Errorf("exchange GitHub authorization code: %w", err)
	}
	user, err := service.provider.AuthenticatedUser(ctx, accessToken)
	if err != nil {
		return LoginResult{}, fmt.Errorf("load authenticated GitHub user: %w", err)
	}
	if user.ID != service.knowledgeOwnerGitHubID {
		return LoginResult{}, ErrNotKnowledgeOwner
	}

	sessionToken, err := service.generateToken()
	if err != nil {
		return LoginResult{}, fmt.Errorf("generate owner session: %w", err)
	}
	session := Session{
		Owner: Owner{
			GitHubID:  user.ID,
			Login:     user.Login,
			AvatarURL: user.AvatarURL,
		},
		ExpiresAt: service.now().Add(sessionLifetime),
	}
	if err := service.store.SaveSession(ctx, tokenDigest(sessionToken), session, session.ExpiresAt); err != nil {
		return LoginResult{}, fmt.Errorf("save owner session: %w", err)
	}

	return LoginResult{SessionToken: sessionToken, Session: session}, nil
}

func (service *Service) Authenticate(ctx context.Context, sessionToken string) (Session, error) {
	if sessionToken == "" {
		return Session{}, ErrUnauthenticated
	}

	session, err := service.store.FindSession(ctx, tokenDigest(sessionToken), service.now())
	if errors.Is(err, ErrSessionNotFound) {
		return Session{}, ErrUnauthenticated
	}
	if err != nil {
		return Session{}, fmt.Errorf("find owner session: %w", err)
	}
	if session.Owner.GitHubID != service.knowledgeOwnerGitHubID {
		return Session{}, ErrUnauthenticated
	}
	return session, nil
}

func (service *Service) Logout(ctx context.Context, sessionToken string) error {
	if sessionToken == "" {
		return nil
	}
	if err := service.store.DeleteSession(ctx, tokenDigest(sessionToken)); err != nil {
		return fmt.Errorf("delete owner session: %w", err)
	}
	return nil
}

func tokenDigest(token string) [32]byte {
	return sha256.Sum256([]byte(token))
}
