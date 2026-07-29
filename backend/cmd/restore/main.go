package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Ken-Chy129/note-foundry/backend/internal/backup"
	"github.com/Ken-Chy129/note-foundry/backend/internal/platform/config"
)

func main() {
	objectKey := flag.String("object", "", "encrypted backup object key; defaults to latest daily backup")
	confirm := flag.Bool("confirm", false, "confirm replacement of the target database and Attachments directory")
	flag.Parse()
	if !*confirm {
		log.Fatal("restore requires -confirm because it replaces database contents and Attachments")
	}
	runtimeConfig, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
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
	backupService := backup.NewService(backup.ServiceConfig{Store: store, Prefix: runtimeConfig.BackupPrefix})
	key := *objectKey
	if key == "" {
		latest, err := backupService.Latest(ctx)
		if err != nil {
			log.Fatal(err)
		}
		key = latest.Key
	}
	source, err := backupService.Open(ctx, key)
	if err != nil {
		log.Fatal(err)
	}
	defer source.Close()
	result, err := (backup.Restorer{
		DatabaseURL:          runtimeConfig.DatabaseURL,
		AttachmentsDirectory: runtimeConfig.AttachmentsDirectory,
		Passphrase:           runtimeConfig.BackupPassphrase,
		Tools: backup.PostgresTools{
			DumpCommand:    runtimeConfig.PostgresDumpCommand,
			RestoreCommand: runtimeConfig.PostgresRestoreCommand,
		},
	}).Restore(ctx, source)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("restored %s created at %s with %d Attachments", key, result.Manifest.CreatedAt, result.Manifest.AttachmentCount)
}
