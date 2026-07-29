package backup

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"
)

type ArchiveCreator interface {
	Create(context.Context, io.Writer, time.Time) (Manifest, error)
}

type Service struct {
	archiver           ArchiveCreator
	store              ObjectStore
	prefix             string
	dailyRetention     int
	weeklyRetention    int
	temporaryDirectory string
}

type ServiceConfig struct {
	Archiver           ArchiveCreator
	Store              ObjectStore
	Prefix             string
	DailyRetention     int
	WeeklyRetention    int
	TemporaryDirectory string
}

type CreateResult struct {
	Manifest Manifest
	Keys     []string
}

func NewService(config ServiceConfig) *Service {
	dailyRetention := config.DailyRetention
	if dailyRetention <= 0 {
		dailyRetention = 7
	}
	weeklyRetention := config.WeeklyRetention
	if weeklyRetention <= 0 {
		weeklyRetention = 8
	}
	return &Service{
		archiver:           config.Archiver,
		store:              config.Store,
		prefix:             strings.Trim(strings.TrimSpace(config.Prefix), "/"),
		dailyRetention:     dailyRetention,
		weeklyRetention:    weeklyRetention,
		temporaryDirectory: config.TemporaryDirectory,
	}
}

func (service *Service) Create(ctx context.Context, scheduledAt time.Time) (CreateResult, error) {
	encrypted, err := os.CreateTemp(service.temporaryDirectory, "notefoundry-*.tar.gz.age")
	if err != nil {
		return CreateResult{}, fmt.Errorf("create encrypted backup file: %w", err)
	}
	encryptedPath := encrypted.Name()
	defer os.Remove(encryptedPath)
	if err := encrypted.Chmod(0o600); err != nil {
		encrypted.Close()
		return CreateResult{}, fmt.Errorf("secure encrypted backup file: %w", err)
	}
	manifest, createErr := service.archiver.Create(ctx, encrypted, scheduledAt)
	closeErr := encrypted.Close()
	if createErr != nil {
		return CreateResult{}, createErr
	}
	if closeErr != nil {
		return CreateResult{}, fmt.Errorf("close encrypted backup file: %w", closeErr)
	}
	info, err := os.Stat(encryptedPath)
	if err != nil {
		return CreateResult{}, fmt.Errorf("inspect encrypted backup file: %w", err)
	}

	dailyKey := service.dailyKey(scheduledAt)
	if err := service.upload(ctx, dailyKey, encryptedPath, info.Size()); err != nil {
		return CreateResult{}, err
	}
	keys := []string{dailyKey}
	if scheduledAt.UTC().Weekday() == time.Sunday {
		weeklyKey := service.weeklyKey(scheduledAt)
		if err := service.upload(ctx, weeklyKey, encryptedPath, info.Size()); err != nil {
			return CreateResult{}, err
		}
		keys = append(keys, weeklyKey)
	}
	if err := service.prune(ctx, service.joinPrefix("daily/"), service.dailyRetention); err != nil {
		return CreateResult{}, err
	}
	if err := service.prune(ctx, service.joinPrefix("weekly/"), service.weeklyRetention); err != nil {
		return CreateResult{}, err
	}
	return CreateResult{Manifest: manifest, Keys: keys}, nil
}

func (service *Service) Latest(ctx context.Context) (Object, error) {
	objects, err := service.store.List(ctx, service.joinPrefix("daily/"))
	if err != nil {
		return Object{}, err
	}
	if len(objects) == 0 {
		return Object{}, ErrNoBackup
	}
	return objects[0], nil
}

func (service *Service) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	return service.store.Get(ctx, key)
}

func (service *Service) upload(ctx context.Context, key, filePath string, size int64) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open encrypted backup for upload: %w", err)
	}
	defer file.Close()
	return service.store.Put(ctx, key, file, size)
}

func (service *Service) prune(ctx context.Context, prefix string, keep int) error {
	objects, err := service.store.List(ctx, prefix)
	if err != nil {
		return err
	}
	if keep >= len(objects) {
		return nil
	}
	for _, object := range objects[keep:] {
		if err := service.store.Delete(ctx, object.Key); err != nil {
			return err
		}
	}
	return nil
}

func (service *Service) dailyKey(at time.Time) string {
	utc := at.UTC()
	return service.joinPrefix(path.Join("daily", utc.Format("2006"), utc.Format("01"), "notefoundry-"+utc.Format("2006-01-02")+".tar.gz.age"))
}

func (service *Service) weeklyKey(at time.Time) string {
	utc := at.UTC()
	year, week := utc.ISOWeek()
	return service.joinPrefix(fmt.Sprintf("weekly/%04d/notefoundry-%04d-W%02d.tar.gz.age", year, year, week))
}

func (service *Service) joinPrefix(value string) string {
	if service.prefix == "" {
		return value
	}
	return path.Join(service.prefix, value)
}
