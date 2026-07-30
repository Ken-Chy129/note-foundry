package config

import (
	"errors"
	"strconv"
	"strings"
	"time"
)

var (
	ErrBackupEndpointRequired   = errors.New("BACKUP_S3_ENDPOINT is required")
	ErrBackupAccessKeyRequired  = errors.New("BACKUP_S3_ACCESS_KEY is required")
	ErrBackupSecretKeyRequired  = errors.New("BACKUP_S3_SECRET_KEY is required")
	ErrBackupBucketRequired     = errors.New("BACKUP_S3_BUCKET is required")
	ErrBackupPassphraseRequired = errors.New("BACKUP_PASSPHRASE is required")
	ErrInvalidBackupEnabled     = errors.New("BACKUP_ENABLED must be true or false")
	ErrInvalidBackupSSL         = errors.New("BACKUP_S3_USE_SSL must be true or false")
	ErrInvalidBackupRetention   = errors.New("backup retention values must be positive integers")
	ErrInvalidWorkerPoll        = errors.New("WORKER_POLL_INTERVAL must be a positive duration")
)

type WorkerConfig struct {
	DatabaseURL              string
	AttachmentsDirectory     string
	WorkerID                 string
	PollInterval             time.Duration
	BackupEnabled            bool
	BackupS3Endpoint         string
	BackupS3AccessKey        string
	BackupS3SecretKey        string
	BackupS3Bucket           string
	BackupS3Region           string
	BackupS3UseSSL           bool
	BackupPrefix             string
	BackupPassphrase         string
	BackupDailyRetention     int
	BackupWeeklyRetention    int
	BackupTemporaryDirectory string
	PostgresDumpCommand      string
	PostgresRestoreCommand   string
}

func LoadWorker(lookup LookupEnv) (WorkerConfig, error) {
	databaseURL, err := LoadDatabaseURL(lookup)
	if err != nil {
		return WorkerConfig{}, err
	}
	endpoint := strings.TrimSpace(valueOrDefault(lookup, "BACKUP_S3_ENDPOINT", ""))
	accessKey := strings.TrimSpace(valueOrDefault(lookup, "BACKUP_S3_ACCESS_KEY", ""))
	secretKey := strings.TrimSpace(valueOrDefault(lookup, "BACKUP_S3_SECRET_KEY", ""))
	bucket := strings.TrimSpace(valueOrDefault(lookup, "BACKUP_S3_BUCKET", ""))
	passphrase := valueOrDefault(lookup, "BACKUP_PASSPHRASE", "")
	backupEnabled, err := strconv.ParseBool(valueOrDefault(lookup, "BACKUP_ENABLED", "false"))
	if err != nil {
		return WorkerConfig{}, ErrInvalidBackupEnabled
	}
	useSSL := true
	dailyRetention := 7
	weeklyRetention := 8
	if backupEnabled {
		if endpoint == "" {
			return WorkerConfig{}, ErrBackupEndpointRequired
		}
		if accessKey == "" {
			return WorkerConfig{}, ErrBackupAccessKeyRequired
		}
		if secretKey == "" {
			return WorkerConfig{}, ErrBackupSecretKeyRequired
		}
		if bucket == "" {
			return WorkerConfig{}, ErrBackupBucketRequired
		}
		if strings.TrimSpace(passphrase) == "" {
			return WorkerConfig{}, ErrBackupPassphraseRequired
		}
		useSSL, err = strconv.ParseBool(valueOrDefault(lookup, "BACKUP_S3_USE_SSL", "true"))
		if err != nil {
			return WorkerConfig{}, ErrInvalidBackupSSL
		}
		dailyRetention, err = positiveInt(valueOrDefault(lookup, "BACKUP_DAILY_RETENTION", "7"))
		if err != nil {
			return WorkerConfig{}, ErrInvalidBackupRetention
		}
		weeklyRetention, err = positiveInt(valueOrDefault(lookup, "BACKUP_WEEKLY_RETENTION", "8"))
		if err != nil {
			return WorkerConfig{}, ErrInvalidBackupRetention
		}
	}
	pollInterval, err := time.ParseDuration(valueOrDefault(lookup, "WORKER_POLL_INTERVAL", "5s"))
	if err != nil || pollInterval <= 0 {
		return WorkerConfig{}, ErrInvalidWorkerPoll
	}
	return WorkerConfig{
		DatabaseURL:              databaseURL,
		AttachmentsDirectory:     valueOrDefault(lookup, "ATTACHMENTS_DIR", "./data/attachments"),
		WorkerID:                 valueOrDefault(lookup, "WORKER_ID", ""),
		PollInterval:             pollInterval,
		BackupEnabled:            backupEnabled,
		BackupS3Endpoint:         endpoint,
		BackupS3AccessKey:        accessKey,
		BackupS3SecretKey:        secretKey,
		BackupS3Bucket:           bucket,
		BackupS3Region:           valueOrDefault(lookup, "BACKUP_S3_REGION", "us-east-1"),
		BackupS3UseSSL:           useSSL,
		BackupPrefix:             valueOrDefault(lookup, "BACKUP_PREFIX", "notefoundry"),
		BackupPassphrase:         passphrase,
		BackupDailyRetention:     dailyRetention,
		BackupWeeklyRetention:    weeklyRetention,
		BackupTemporaryDirectory: valueOrDefault(lookup, "BACKUP_TEMP_DIR", ""),
		PostgresDumpCommand:      valueOrDefault(lookup, "PG_DUMP_COMMAND", "pg_dump"),
		PostgresRestoreCommand:   valueOrDefault(lookup, "PG_RESTORE_COMMAND", "pg_restore"),
	}, nil
}

func positiveInt(value string) (int, error) {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed <= 0 {
		return 0, ErrInvalidBackupRetention
	}
	return parsed, nil
}
