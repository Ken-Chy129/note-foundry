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
	"github.com/Ken-Chy129/note-foundry/backend/internal/historyimport"
	"github.com/Ken-Chy129/note-foundry/backend/internal/knowledge"
	"github.com/Ken-Chy129/note-foundry/backend/internal/notes"
	"github.com/Ken-Chy129/note-foundry/backend/internal/platform/config"
	"github.com/Ken-Chy129/note-foundry/backend/internal/platform/database"
	searchmodule "github.com/Ken-Chy129/note-foundry/backend/internal/search"
	"github.com/google/uuid"
)

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout, os.LookupEnv); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, arguments []string, stdout io.Writer, lookupEnv config.LookupEnv) error {
	flags := flag.NewFlagSet("historyrecover", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	bundleDirectory := flags.String("bundle", "", "validated high-confidence asset recovery bundle")
	apply := flags.Bool("apply", false, "upload recovered assets and update draft Markdown")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if strings.TrimSpace(*bundleDirectory) == "" {
		return errors.New("-bundle is required")
	}

	validated, err := historyimport.ValidateRecoveryBundle(*bundleDirectory)
	if err != nil {
		return err
	}
	if !*apply {
		_, err := fmt.Fprintf(stdout, "preflight=ok documents=%d attachments=%d confidence=%s bundle_sha256=%s\n",
			len(validated.Manifest.Documents),
			validated.AttachmentCount,
			historyimport.RecoveryConfidenceHigh,
			validated.BundleSHA256Hex,
		)
		return err
	}

	databaseURL, err := config.LoadDatabaseURL(lookupEnv)
	if err != nil {
		return err
	}
	attachmentsDirectory, ok := lookupEnv("ATTACHMENTS_DIR")
	attachmentsDirectory = strings.TrimSpace(attachmentsDirectory)
	if !ok || attachmentsDirectory == "" {
		return errors.New("ATTACHMENTS_DIR is required when -apply is set")
	}
	startupContext, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	pool, err := database.Open(startupContext, databaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

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

	summary, err := historyimport.ApplyRecovery(ctx, *bundleDirectory, notesService, attachmentService)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "apply=ok documents=%d attachments=%d attachments_uploaded=%d documents_completed=%d state=%s\n",
		summary.DocumentsPlanned,
		summary.AttachmentsPlanned,
		summary.AttachmentsUploaded,
		summary.DocumentsCompleted,
		summary.StatePath,
	)
	return err
}
