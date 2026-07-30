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
	flags := flag.NewFlagSet("historyapply", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	stagingDirectory := flags.String("staging", "", "validated history staging directory")
	apply := flags.Bool("apply", false, "write the staging package to the configured runtime")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if strings.TrimSpace(*stagingDirectory) == "" {
		return errors.New("-staging is required")
	}

	validated, err := historyimport.ValidateStaging(*stagingDirectory)
	if err != nil {
		return err
	}
	if !*apply {
		_, err := fmt.Fprintf(stdout, "preflight=ok space=%q visibility=%s documents=%d attachments=%d staging_sha256=%s\n",
			validated.Manifest.Space.Name,
			validated.Manifest.Space.Visibility,
			len(validated.Manifest.Documents),
			validated.AttachmentCount,
			validated.StagingSHA256Hex,
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

	summary, err := historyimport.ApplyStaging(ctx, *stagingDirectory, knowledgeService, notesService, attachmentService)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "apply=ok documents=%d attachments=%d spaces_created=%d directories_created=%d notes_created=%d attachments_uploaded=%d documents_completed=%d state=%s\n",
		summary.DocumentsPlanned,
		summary.AttachmentsPlanned,
		summary.SpacesCreated,
		summary.DirectoriesCreated,
		summary.NotesCreated,
		summary.AttachmentsUploaded,
		summary.DocumentsCompleted,
		summary.StatePath,
	)
	return err
}
