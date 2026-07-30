package jobs

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/Ken-Chy129/note-foundry/backend/internal/platform/database"
)

func TestPostgresRepositoryClaimsRetriesAndDeduplicatesJobs(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool, err := database.Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("database.Open() error = %v", err)
	}
	defer pool.Close()
	if err := database.ApplyMigrations(ctx, pool); err != nil {
		t.Fatalf("ApplyMigrations() error = %v", err)
	}
	if _, err := pool.Exec(ctx, `TRUNCATE jobs`); err != nil {
		t.Fatalf("truncate Jobs: %v", err)
	}

	repository := NewPostgresRepository(pool)
	now := time.Date(2026, 7, 29, 2, 0, 0, 0, time.UTC)
	job, err := New("11111111-1111-4111-8111-111111111111", "backup.create", nil, 2, now, "backup:daily:2026-07-29")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	stored, inserted, err := repository.Enqueue(ctx, job)
	if err != nil || !inserted {
		t.Fatalf("Enqueue() = %+v, %t, %v", stored, inserted, err)
	}
	duplicate, inserted, err := repository.Enqueue(ctx, Job{
		ID:             "22222222-2222-4222-8222-222222222222",
		Kind:           job.Kind,
		Payload:        job.Payload,
		MaxAttempts:    job.MaxAttempts,
		AvailableAt:    job.AvailableAt,
		IdempotencyKey: job.IdempotencyKey,
	})
	if err != nil || inserted || duplicate.ID != stored.ID {
		t.Fatalf("duplicate Enqueue() = %+v, %t, %v", duplicate, inserted, err)
	}
	urlJob, _ := New("33333333-3333-4333-8333-333333333333", "source.extract.url", nil, 3, now, "source:11111111-1111-4111-8111-111111111111:url")
	if _, inserted, err := repository.Enqueue(ctx, urlJob); err != nil || !inserted {
		t.Fatalf("enqueue URL extraction Job = %t, %v", inserted, err)
	}

	claimed, err := repository.Claim(ctx, "worker-1", now, 5*time.Minute, []string{"backup.create"})
	if err != nil {
		t.Fatalf("first Claim() error = %v", err)
	}
	if claimed.Attempts != 1 || claimed.State != StateRunning {
		t.Errorf("first claimed Job = %+v", claimed)
	}
	if _, err := repository.Claim(ctx, "worker-2", now, 5*time.Minute, []string{"backup.create"}); !errors.Is(err, ErrNoJob) {
		t.Fatalf("concurrent Claim() error = %v, want %v", err, ErrNoJob)
	}

	retryAt := now.Add(time.Minute)
	state, err := repository.Fail(ctx, claimed.ID, "worker-1", now, retryAt, "temporary failure")
	if err != nil || state != StatePending {
		t.Fatalf("first Fail() = %q, %v", state, err)
	}
	if _, err := repository.Claim(ctx, "worker-2", now, 5*time.Minute, []string{"backup.create"}); !errors.Is(err, ErrNoJob) {
		t.Fatalf("early retry Claim() error = %v, want %v", err, ErrNoJob)
	}
	claimed, err = repository.Claim(ctx, "worker-2", retryAt, 5*time.Minute, []string{"backup.create"})
	if err != nil || claimed.Attempts != 2 {
		t.Fatalf("retry Claim() = %+v, %v", claimed, err)
	}
	state, err = repository.Fail(ctx, claimed.ID, "worker-2", retryAt, retryAt.Add(time.Hour), "permanent failure")
	if err != nil || state != StateFailed {
		t.Fatalf("final Fail() = %q, %v", state, err)
	}
	if _, err := repository.Claim(ctx, "worker-3", retryAt.Add(2*time.Hour), 5*time.Minute, []string{"backup.create"}); !errors.Is(err, ErrNoJob) {
		t.Fatalf("failed Job Claim() error = %v, want %v", err, ErrNoJob)
	}
	urlClaim, err := repository.Claim(ctx, "source-worker", retryAt.Add(2*time.Hour), 5*time.Minute, []string{"source.extract.url"})
	if err != nil || urlClaim.ID != urlJob.ID {
		t.Fatalf("URL extraction Claim() = %+v, %v", urlClaim, err)
	}
}

func TestPostgresRepositoryReclaimsExpiredWorkerLease(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool, err := database.Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("database.Open() error = %v", err)
	}
	defer pool.Close()
	if err := database.ApplyMigrations(ctx, pool); err != nil {
		t.Fatalf("ApplyMigrations() error = %v", err)
	}
	if _, err := pool.Exec(ctx, `TRUNCATE jobs`); err != nil {
		t.Fatalf("truncate Jobs: %v", err)
	}
	repository := NewPostgresRepository(pool)
	now := time.Date(2026, 7, 29, 3, 0, 0, 0, time.UTC)
	job, _ := New("44444444-4444-4444-8444-444444444444", "backup.create", nil, 3, now, "")
	if _, _, err := repository.Enqueue(ctx, job); err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}
	if _, err := repository.Claim(ctx, "worker-1", now, 5*time.Minute, []string{"backup.create"}); err != nil {
		t.Fatalf("first Claim() error = %v", err)
	}
	reclaimed, err := repository.Claim(ctx, "worker-2", now.Add(6*time.Minute), 5*time.Minute, []string{"backup.create"})
	if err != nil {
		t.Fatalf("reclaim expired Job error = %v", err)
	}
	if reclaimed.Attempts != 2 || reclaimed.LockedBy != "worker-2" {
		t.Errorf("reclaimed Job = %+v", reclaimed)
	}
}
