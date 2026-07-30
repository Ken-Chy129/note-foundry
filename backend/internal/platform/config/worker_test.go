package config

import (
	"errors"
	"testing"
	"time"
)

func TestLoadWorkerUsesSafeDefaults(t *testing.T) {
	values := map[string]string{
		"DATABASE_URL":         "postgres://localhost/notefoundry",
		"BACKUP_ENABLED":       "true",
		"BACKUP_S3_ENDPOINT":   "localhost:9000",
		"BACKUP_S3_ACCESS_KEY": "access",
		"BACKUP_S3_SECRET_KEY": "secret",
		"BACKUP_S3_BUCKET":     "notefoundry",
		"BACKUP_PASSPHRASE":    "test backup passphrase",
	}
	config, err := LoadWorker(func(key string) (string, bool) { value, ok := values[key]; return value, ok })
	if err != nil {
		t.Fatalf("LoadWorker() error = %v", err)
	}
	if !config.BackupEnabled || !config.BackupS3UseSSL || config.BackupDailyRetention != 7 || config.BackupWeeklyRetention != 8 || config.PollInterval != 5*time.Second {
		t.Errorf("WorkerConfig defaults = %+v", config)
	}
}

func TestLoadWorkerAllowsBackupToBeDisabled(t *testing.T) {
	values := map[string]string{"DATABASE_URL": "postgres://localhost/notefoundry"}
	config, err := LoadWorker(func(key string) (string, bool) { value, ok := values[key]; return value, ok })
	if err != nil {
		t.Fatalf("LoadWorker() error = %v", err)
	}
	if config.BackupEnabled {
		t.Fatal("backup is enabled without S3 configuration")
	}
}

func TestLoadWorkerRejectsPartialBackupCredentials(t *testing.T) {
	values := map[string]string{
		"DATABASE_URL":       "postgres://localhost/notefoundry",
		"BACKUP_ENABLED":     "true",
		"BACKUP_S3_ENDPOINT": "localhost:9000",
	}
	_, err := LoadWorker(func(key string) (string, bool) { value, ok := values[key]; return value, ok })
	if !errors.Is(err, ErrBackupAccessKeyRequired) {
		t.Fatalf("LoadWorker() error = %v, want %v", err, ErrBackupAccessKeyRequired)
	}
}
