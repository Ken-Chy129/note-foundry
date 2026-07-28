package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Ken-Chy129/note-foundry/backend/internal/identity"
	"github.com/Ken-Chy129/note-foundry/backend/internal/platform/config"
	"github.com/Ken-Chy129/note-foundry/backend/internal/platform/database"
	"github.com/Ken-Chy129/note-foundry/backend/internal/platform/httpapi"
)

func main() {
	runtimeConfig, err := config.Load(os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}

	startupContext, cancelStartup := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelStartup()

	pool, err := database.Open(startupContext, runtimeConfig.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	identityStore := identity.NewPostgresStore(pool)
	githubProvider := identity.NewGitHubProvider(identity.GitHubProviderConfig{
		ClientID:     runtimeConfig.GitHubClientID,
		ClientSecret: runtimeConfig.GitHubClientSecret,
		RedirectURL:  runtimeConfig.PublicURL + "/auth/github/callback",
	})
	identityService := identity.NewService(identity.ServiceConfig{
		KnowledgeOwnerGitHubID: runtimeConfig.GitHubOwnerID,
		Store:                  identityStore,
		Provider:               githubProvider,
		GenerateToken:          identity.GenerateSecureToken,
		Now:                    time.Now,
	})
	identityHTTP := identity.NewHTTPHandler(identityService, identity.HTTPConfig{
		SecureCookies: runtimeConfig.SecureCookies,
		PostLoginPath: "/app",
	})

	server := &http.Server{
		Addr:              runtimeConfig.HTTPAddress,
		Handler:           newHandler(pool, identityHTTP),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("NoteFoundry API listening on %s", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

type readinessChecker interface {
	Ping(context.Context) error
}

type routeRegistrar interface {
	RegisterRoutes(*http.ServeMux)
}

func newHandler(readiness readinessChecker, routes routeRegistrar) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(response http.ResponseWriter, _ *http.Request) {
		_ = httpapi.WriteJSON(response, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", func(response http.ResponseWriter, request *http.Request) {
		ctx, cancel := context.WithTimeout(request.Context(), time.Second)
		defer cancel()

		if err := readiness.Ping(ctx); err != nil {
			httpapi.WriteError(response, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "service is not ready")
			return
		}
		_ = httpapi.WriteJSON(response, http.StatusOK, map[string]string{"status": "ready"})
	})
	if routes != nil {
		routes.RegisterRoutes(mux)
	}
	return mux
}
