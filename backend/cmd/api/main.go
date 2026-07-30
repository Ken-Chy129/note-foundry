package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Ken-Chy129/note-foundry/backend/internal/attachments"
	"github.com/Ken-Chy129/note-foundry/backend/internal/identity"
	"github.com/Ken-Chy129/note-foundry/backend/internal/knowledge"
	"github.com/Ken-Chy129/note-foundry/backend/internal/notes"
	"github.com/Ken-Chy129/note-foundry/backend/internal/platform/config"
	"github.com/Ken-Chy129/note-foundry/backend/internal/platform/database"
	"github.com/Ken-Chy129/note-foundry/backend/internal/platform/httpapi"
	searchmodule "github.com/Ken-Chy129/note-foundry/backend/internal/search"
	"github.com/google/uuid"
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
		SecureCookies:     runtimeConfig.SecureCookies,
		PostLoginPath:     "/workspace",
		TrustForwardedFor: runtimeConfig.Environment == config.EnvironmentProduction,
	})
	knowledgeRepository := knowledge.NewPostgresRepository(pool)
	searchRepository := searchmodule.NewPostgresRepository(pool)
	searchService := searchmodule.NewService(searchRepository)
	searchHTTP := searchmodule.NewHTTPHandler(searchService, identityHTTP.RequireOwner)
	attachmentStorage, err := attachments.NewLocalStorage(runtimeConfig.AttachmentsDirectory)
	if err != nil {
		log.Fatal(err)
	}
	attachmentRepository := attachments.NewPostgresRepository(pool)
	attachmentService := attachments.NewService(attachmentRepository, attachmentStorage, uuid.NewString, time.Now)
	attachmentHTTP := attachments.NewHTTPHandler(attachmentService, identityHTTP.RequireOwner)
	knowledgeService := knowledge.NewService(knowledge.ServiceConfig{
		Spaces:      knowledgeRepository,
		Directories: knowledgeRepository,
		Tags:        knowledgeRepository,
		Links:       knowledgeRepository,
		Search:      searchService,
		GenerateID:  uuid.NewString,
	})
	knowledgeHTTP := knowledge.NewHTTPHandler(knowledgeService, identityHTTP.RequireOwner)
	notesRepository := notes.NewPostgresRepository(pool)
	notesService := notes.NewService(notes.ServiceConfig{
		Notes:       notesRepository,
		Knowledge:   knowledgeService,
		GenerateID:  uuid.NewString,
		Now:         time.Now,
		Links:       knowledgeService,
		Search:      searchService,
		Attachments: attachmentService,
	})
	notesHTTP := notes.NewHTTPHandler(notesService, identityHTTP.RequireOwner)

	server := &http.Server{
		Addr:              runtimeConfig.HTTPAddress,
		Handler:           httpapi.SecurityHeaders(newHandler(pool, identityHTTP, knowledgeHTTP, notesHTTP, searchHTTP, attachmentHTTP), runtimeConfig.SecureCookies),
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

func newHandler(readiness readinessChecker, routes ...routeRegistrar) http.Handler {
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
	for _, registrar := range routes {
		if registrar != nil {
			registrar.RegisterRoutes(mux)
		}
	}
	return mux
}
