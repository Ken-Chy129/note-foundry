package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/Ken-Chy129/note-foundry/backend/internal/attachments"
	"github.com/Ken-Chy129/note-foundry/backend/internal/knowledge"
	"github.com/Ken-Chy129/note-foundry/backend/internal/notes"
	"github.com/Ken-Chy129/note-foundry/backend/internal/platform/config"
	"github.com/Ken-Chy129/note-foundry/backend/internal/platform/database"
	searchmodule "github.com/Ken-Chy129/note-foundry/backend/internal/search"
	"github.com/google/uuid"
)

const defaultLimit = 1000

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout, os.LookupEnv); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, arguments []string, stdout io.Writer, lookupEnv config.LookupEnv) error {
	flags := flag.NewFlagSet("publishpublic", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	confirm := flags.Bool("confirm", false, "publish every listed draft in a public Knowledge Space")
	limit := flags.Int("limit", defaultLimit, "maximum number of drafts to inspect and publish")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if *limit <= 0 {
		return notes.ErrInvalidBulkPublishLimit
	}

	databaseURL, err := config.LoadDatabaseURL(lookupEnv)
	if err != nil {
		return err
	}
	attachmentsDirectory, ok := lookupEnv("ATTACHMENTS_DIR")
	attachmentsDirectory = strings.TrimSpace(attachmentsDirectory)
	if !ok || attachmentsDirectory == "" {
		return errors.New("ATTACHMENTS_DIR is required")
	}

	operationContext, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	pool, err := database.Open(operationContext, databaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := database.ApplyMigrations(operationContext, pool); err != nil {
		return err
	}

	knowledgeRepository := knowledge.NewPostgresRepository(pool)
	searchRepository := searchmodule.NewPostgresRepository(pool)
	searchService := searchmodule.NewService(searchRepository)
	attachmentStorage, err := attachments.NewLocalStorage(attachmentsDirectory)
	if err != nil {
		return err
	}
	attachmentRepository := attachments.NewPostgresRepository(pool)
	attachmentService := attachments.NewService(attachmentRepository, attachmentStorage, uuid.NewString, time.Now)
	knowledgeService := knowledge.NewService(knowledge.ServiceConfig{
		Spaces:      knowledgeRepository,
		Directories: knowledgeRepository,
		Tags:        knowledgeRepository,
		Links:       knowledgeRepository,
		Search:      searchService,
		GenerateID:  uuid.NewString,
	})
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

	candidates, err := notesRepository.ListUnpublishedPublicNotes(operationContext, *limit)
	if err != nil {
		return err
	}
	for _, candidate := range candidates {
		if _, err := fmt.Fprintf(stdout, "candidate space=%q note=%q version=%d\n", candidate.SpaceName, candidate.Title, candidate.Version); err != nil {
			return err
		}
	}
	if !*confirm {
		_, err := fmt.Fprintf(stdout, "preflight=ok candidates=%d confirm_required=true\n", len(candidates))
		return err
	}

	result, err := notes.PublishPublicDrafts(operationContext, notesRepository, notesService, *limit)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "publish=ok candidates=%d published=%d\n", result.Candidates, result.Published)
	return err
}
