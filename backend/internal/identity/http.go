package identity

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/Ken-Chy129/note-foundry/backend/internal/platform/httpapi"
)

const (
	OAuthStateCookieName = "notefoundry_oauth_state"
	SessionCookieName    = "notefoundry_session"
)

type HTTPConfig struct {
	SecureCookies        bool
	PostLoginPath        string
	AuthenticationLimit  int
	AuthenticationWindow time.Duration
	TrustForwardedFor    bool
}

type HTTPHandler struct {
	service             *Service
	secureCookies       bool
	postLoginPath       string
	authenticationLimit *httpapi.FixedWindowLimiter
}

func NewHTTPHandler(service *Service, config HTTPConfig) *HTTPHandler {
	postLoginPath := config.PostLoginPath
	if postLoginPath == "" {
		postLoginPath = "/workspace"
	}
	return &HTTPHandler{
		service:       service,
		secureCookies: config.SecureCookies,
		postLoginPath: postLoginPath,
		authenticationLimit: httpapi.NewFixedWindowLimiter(httpapi.FixedWindowConfig{
			Limit:             config.AuthenticationLimit,
			Window:            config.AuthenticationWindow,
			TrustForwardedFor: config.TrustForwardedFor,
		}),
	}
}

func (handler *HTTPHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle("GET /auth/github/start", handler.authenticationLimit.Middleware(http.HandlerFunc(handler.startGitHubLogin)))
	mux.Handle("GET /auth/github/callback", handler.authenticationLimit.Middleware(http.HandlerFunc(handler.completeGitHubLogin)))
	mux.HandleFunc("GET /api/v1/session", handler.getSession)
	mux.HandleFunc("POST /auth/logout", handler.logout)
}

func (handler *HTTPHandler) startGitHubLogin(response http.ResponseWriter, request *http.Request) {
	start, err := handler.service.StartLogin(request.Context())
	if err != nil {
		httpapi.WriteError(response, http.StatusInternalServerError, "INTERNAL_ERROR", "could not start authentication")
		return
	}

	http.SetCookie(response, &http.Cookie{
		Name:     OAuthStateCookieName,
		Value:    start.State,
		Path:     "/auth/github/callback",
		Expires:  start.ExpiresAt,
		MaxAge:   int(oauthStateLifetime.Seconds()),
		HttpOnly: true,
		Secure:   handler.secureCookies,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(response, request, start.AuthorizationURL, http.StatusFound)
}

func (handler *HTTPHandler) completeGitHubLogin(response http.ResponseWriter, request *http.Request) {
	browserState := ""
	if cookie, err := request.Cookie(OAuthStateCookieName); err == nil {
		browserState = cookie.Value
	}
	handler.clearOAuthStateCookie(response)

	result, err := handler.service.CompleteLogin(
		request.Context(),
		browserState,
		request.URL.Query().Get("state"),
		request.URL.Query().Get("code"),
	)
	if errors.Is(err, ErrInvalidOAuthState) || errors.Is(err, ErrAuthorizationCodeRequired) {
		httpapi.WriteError(response, http.StatusBadRequest, "INVALID_OAUTH_CALLBACK", "GitHub authentication could not be completed")
		return
	}
	if errors.Is(err, ErrNotKnowledgeOwner) {
		httpapi.WriteError(response, http.StatusForbidden, "FORBIDDEN", "GitHub account is not authorized")
		return
	}
	if err != nil {
		httpapi.WriteError(response, http.StatusBadGateway, "AUTHENTICATION_FAILED", "GitHub authentication failed")
		return
	}

	http.SetCookie(response, &http.Cookie{
		Name:     SessionCookieName,
		Value:    result.SessionToken,
		Path:     "/",
		Expires:  result.Session.ExpiresAt,
		MaxAge:   int(sessionLifetime.Seconds()),
		HttpOnly: true,
		Secure:   handler.secureCookies,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(response, request, handler.postLoginPath, http.StatusFound)
}

func (handler *HTTPHandler) getSession(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Cache-Control", "no-store")
	session, err := handler.authenticateRequest(request)
	if errors.Is(err, ErrUnauthenticated) {
		httpapi.WriteError(response, http.StatusUnauthorized, "UNAUTHENTICATED", "owner session is required")
		return
	}
	if err != nil {
		httpapi.WriteError(response, http.StatusInternalServerError, "INTERNAL_ERROR", "could not load owner session")
		return
	}

	_ = httpapi.WriteJSON(response, http.StatusOK, map[string]any{
		"owner": map[string]any{
			"githubUserId": session.Owner.GitHubID,
			"login":        session.Owner.Login,
			"avatarUrl":    session.Owner.AvatarURL,
		},
		"expiresAt": session.ExpiresAt,
	})
}

func (handler *HTTPHandler) logout(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Cache-Control", "no-store")
	sessionToken := ""
	if cookie, err := request.Cookie(SessionCookieName); err == nil {
		sessionToken = cookie.Value
	}
	if err := handler.service.Logout(request.Context(), sessionToken); err != nil {
		httpapi.WriteError(response, http.StatusInternalServerError, "INTERNAL_ERROR", "could not end owner session")
		return
	}
	handler.clearSessionCookie(response)
	response.WriteHeader(http.StatusNoContent)
}

func (handler *HTTPHandler) RequireOwner(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		session, err := handler.authenticateRequest(request)
		if errors.Is(err, ErrUnauthenticated) {
			httpapi.WriteError(response, http.StatusUnauthorized, "UNAUTHENTICATED", "owner session is required")
			return
		}
		if err != nil {
			httpapi.WriteError(response, http.StatusInternalServerError, "INTERNAL_ERROR", "could not verify owner session")
			return
		}

		ctx := context.WithValue(request.Context(), sessionContextKey{}, session)
		next.ServeHTTP(response, request.WithContext(ctx))
	})
}

func (handler *HTTPHandler) authenticateRequest(request *http.Request) (Session, error) {
	cookie, err := request.Cookie(SessionCookieName)
	if err != nil {
		return Session{}, ErrUnauthenticated
	}
	return handler.service.Authenticate(request.Context(), cookie.Value)
}

func (handler *HTTPHandler) clearOAuthStateCookie(response http.ResponseWriter) {
	http.SetCookie(response, &http.Cookie{
		Name:     OAuthStateCookieName,
		Path:     "/auth/github/callback",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   handler.secureCookies,
		SameSite: http.SameSiteLaxMode,
	})
}

func (handler *HTTPHandler) clearSessionCookie(response http.ResponseWriter) {
	http.SetCookie(response, &http.Cookie{
		Name:     SessionCookieName,
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   handler.secureCookies,
		SameSite: http.SameSiteLaxMode,
	})
}

type sessionContextKey struct{}

func SessionFromContext(ctx context.Context) (Session, bool) {
	session, ok := ctx.Value(sessionContextKey{}).(Session)
	return session, ok
}
