package backup

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sort"
	"testing"
	"time"

	"github.com/Ken-Chy129/note-foundry/backend/internal/jobs"
)

func TestServiceUploadsDailyAndWeeklyBackupsAndPrunesRetention(t *testing.T) {
	store := newMemoryStore()
	for index := 1; index <= 3; index++ {
		key := "notefoundry/daily/2026/07/old-" + string(rune('0'+index))
		store.objects[key] = memoryObject{contents: []byte("old"), modifiedAt: time.Date(2026, 7, index, 0, 0, 0, 0, time.UTC)}
	}
	service := NewService(ServiceConfig{
		Archiver:        archiveCreatorStub{},
		Store:           store,
		Prefix:          "notefoundry",
		DailyRetention:  2,
		WeeklyRetention: 2,
	})
	sunday := time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)
	result, err := service.Create(context.Background(), sunday)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if len(result.Keys) != 2 {
		t.Fatalf("uploaded keys = %v", result.Keys)
	}
	if _, ok := store.objects["notefoundry/daily/2026/08/notefoundry-2026-08-02.tar.gz.age"]; !ok {
		t.Errorf("daily backup was not uploaded: %v", store.objects)
	}
	if _, ok := store.objects["notefoundry/weekly/2026/notefoundry-2026-W31.tar.gz.age"]; !ok {
		t.Errorf("weekly backup was not uploaded: %v", store.objects)
	}
	daily, err := store.List(context.Background(), "notefoundry/daily/")
	if err != nil {
		t.Fatal(err)
	}
	if len(daily) != 2 {
		t.Errorf("daily retention count = %d, want 2", len(daily))
	}
}

func TestSchedulerEnqueuesOneIdempotentJobPerUTCDay(t *testing.T) {
	enqueuer := &enqueuerStub{}
	scheduler := NewScheduler(enqueuer, func() string { return "11111111-1111-4111-8111-111111111111" })
	now := time.Date(2026, 7, 29, 18, 30, 0, 0, time.FixedZone("CST", 8*60*60))
	job, inserted, err := scheduler.EnsureDaily(context.Background(), now)
	if err != nil || !inserted {
		t.Fatalf("EnsureDaily() = %+v, %t, %v", job, inserted, err)
	}
	if job.IdempotencyKey != "backup:daily:2026-07-29" {
		t.Errorf("idempotency key = %q", job.IdempotencyKey)
	}
	payload, err := DecodeCreatePayload(job)
	if err != nil {
		t.Fatal(err)
	}
	if payload.ScheduledAt.Hour() != 0 || payload.ScheduledAt.Location() != time.UTC {
		t.Errorf("payload = %+v", payload)
	}
}

type archiveCreatorStub struct{}

func (archiveCreatorStub) Create(_ context.Context, destination io.Writer, at time.Time) (Manifest, error) {
	_, err := destination.Write([]byte("encrypted backup"))
	return Manifest{FormatVersion: FormatVersion, CreatedAt: at}, err
}

type memoryObject struct {
	contents   []byte
	modifiedAt time.Time
}

type memoryStore struct {
	objects map[string]memoryObject
}

func newMemoryStore() *memoryStore {
	return &memoryStore{objects: make(map[string]memoryObject)}
}

func (store *memoryStore) Put(_ context.Context, key string, source io.Reader, _ int64) error {
	contents, err := io.ReadAll(source)
	if err != nil {
		return err
	}
	store.objects[key] = memoryObject{contents: contents, modifiedAt: time.Now()}
	return nil
}

func (store *memoryStore) Get(_ context.Context, key string) (io.ReadCloser, error) {
	object, ok := store.objects[key]
	if !ok {
		return nil, errors.New("not found")
	}
	return io.NopCloser(bytes.NewReader(object.contents)), nil
}

func (store *memoryStore) List(_ context.Context, prefix string) ([]Object, error) {
	var objects []Object
	for key, object := range store.objects {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			objects = append(objects, Object{Key: key, Size: int64(len(object.contents)), LastModified: object.modifiedAt})
		}
	}
	sort.Slice(objects, func(left, right int) bool { return objects[left].LastModified.After(objects[right].LastModified) })
	return objects, nil
}

func (store *memoryStore) Delete(_ context.Context, key string) error {
	delete(store.objects, key)
	return nil
}

type enqueuerStub struct {
	job jobs.Job
}

func (enqueuer *enqueuerStub) Enqueue(_ context.Context, job jobs.Job) (jobs.Job, bool, error) {
	enqueuer.job = job
	return job, true, nil
}
