package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Ken-Chy129/note-foundry/backend/internal/backup"
	"github.com/Ken-Chy129/note-foundry/backend/internal/jobs"
	"github.com/Ken-Chy129/note-foundry/backend/internal/platform/config"
	"github.com/Ken-Chy129/note-foundry/backend/internal/platform/database"
	"github.com/google/uuid"
)

func main() {
	runOnce := flag.Bool("once", false, "enqueue today's backup and process available Jobs before exiting")
	flag.Parse()

	runtimeConfig, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := database.Open(ctx, runtimeConfig.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	if err := database.ApplyMigrations(ctx, pool); err != nil {
		log.Fatal(err)
	}
	store, err := backup.NewMinioStore(backup.MinioStoreConfig{
		Endpoint:  runtimeConfig.BackupS3Endpoint,
		AccessKey: runtimeConfig.BackupS3AccessKey,
		SecretKey: runtimeConfig.BackupS3SecretKey,
		Bucket:    runtimeConfig.BackupS3Bucket,
		Region:    runtimeConfig.BackupS3Region,
		UseSSL:    runtimeConfig.BackupS3UseSSL,
	})
	if err != nil {
		log.Fatal(err)
	}
	if err := store.CheckBucket(ctx); err != nil {
		log.Fatal(err)
	}

	jobRepository := jobs.NewPostgresRepository(pool)
	backupService := backup.NewService(backup.ServiceConfig{
		Archiver: backup.Archiver{
			DatabaseURL:          runtimeConfig.DatabaseURL,
			AttachmentsDirectory: runtimeConfig.AttachmentsDirectory,
			Passphrase:           runtimeConfig.BackupPassphrase,
			TemporaryDirectory:   runtimeConfig.BackupTemporaryDirectory,
			Tools: backup.PostgresTools{
				DumpCommand:    runtimeConfig.PostgresDumpCommand,
				RestoreCommand: runtimeConfig.PostgresRestoreCommand,
			},
		},
		Store:              store,
		Prefix:             runtimeConfig.BackupPrefix,
		DailyRetention:     runtimeConfig.BackupDailyRetention,
		WeeklyRetention:    runtimeConfig.BackupWeeklyRetention,
		TemporaryDirectory: runtimeConfig.BackupTemporaryDirectory,
	})
	workerID := runtimeConfig.WorkerID
	if workerID == "" {
		hostname, _ := os.Hostname()
		workerID = fmt.Sprintf("%s-%s", hostname, uuid.NewString())
	}
	runner := jobs.NewRunner(jobs.RunnerConfig{
		Repository: jobRepository,
		WorkerID:   workerID,
		Handlers: map[string]jobs.Handler{
			backup.JobKindCreate: func(ctx context.Context, job jobs.Job) error {
				payload, err := backup.DecodeCreatePayload(job)
				if err != nil {
					return err
				}
				result, err := backupService.Create(ctx, payload.ScheduledAt)
				if err == nil {
					log.Printf("encrypted backup uploaded: %v", result.Keys)
				}
				return err
			},
		},
	})
	scheduler := backup.NewScheduler(jobRepository, uuid.NewString)
	if _, inserted, err := scheduler.EnsureDaily(ctx, time.Now()); err != nil {
		log.Fatal(err)
	} else if inserted {
		log.Print("scheduled today's encrypted backup")
	}

	if *runOnce {
		if err := drainAvailable(ctx, runner); err != nil {
			log.Fatal(err)
		}
		return
	}
	log.Printf("NoteFoundry worker %s started", workerID)
	pollTicker := time.NewTicker(runtimeConfig.PollInterval)
	defer pollTicker.Stop()
	scheduleTicker := time.NewTicker(time.Hour)
	defer scheduleTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-scheduleTicker.C:
			if _, inserted, err := scheduler.EnsureDaily(ctx, time.Now()); err != nil {
				log.Printf("schedule backup: %v", err)
			} else if inserted {
				log.Print("scheduled today's encrypted backup")
			}
		case <-pollTicker.C:
			result, err := runner.RunOnce(ctx)
			if errors.Is(err, jobs.ErrNoJob) {
				continue
			}
			if err != nil {
				log.Printf("run Job: %v", err)
				continue
			}
			if result.HandlerError != nil {
				log.Printf("Job %s failed (%s): %v", result.Job.ID, result.State, result.HandlerError)
			}
		}
	}
}

func drainAvailable(ctx context.Context, runner *jobs.Runner) error {
	for {
		result, err := runner.RunOnce(ctx)
		if errors.Is(err, jobs.ErrNoJob) {
			return nil
		}
		if err != nil {
			return err
		}
		if result.HandlerError != nil {
			return result.HandlerError
		}
	}
}
