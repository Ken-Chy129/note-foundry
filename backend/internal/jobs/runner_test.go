package jobs

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRunnerCompletesSuccessfulJob(t *testing.T) {
	now := time.Date(2026, 7, 29, 1, 0, 0, 0, time.UTC)
	repository := &repositoryStub{claimed: Job{ID: "job-1", Kind: "backup.create", Attempts: 1}}
	runner := NewRunner(RunnerConfig{
		Repository: repository,
		WorkerID:   "worker-1",
		Now:        func() time.Time { return now },
		Handlers: map[string]Handler{
			"backup.create": func(context.Context, Job) error { return nil },
		},
	})

	result, err := runner.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if result.State != StateSucceeded || repository.completedID != "job-1" {
		t.Errorf("result = %+v, completed = %q", result, repository.completedID)
	}
}

func TestRunnerRetriesFailedJobWithExponentialBackoff(t *testing.T) {
	now := time.Date(2026, 7, 29, 1, 0, 0, 0, time.UTC)
	repository := &repositoryStub{claimed: Job{ID: "job-2", Kind: "backup.create", Attempts: 3}, failState: StatePending}
	runner := NewRunner(RunnerConfig{
		Repository:  repository,
		WorkerID:    "worker-1",
		Now:         func() time.Time { return now },
		BaseBackoff: time.Minute,
		MaxBackoff:  time.Hour,
		Handlers: map[string]Handler{
			"backup.create": func(context.Context, Job) error { return errors.New("upload failed") },
		},
	})

	result, err := runner.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if result.State != StatePending || result.HandlerError == nil {
		t.Errorf("result = %+v", result)
	}
	if want := now.Add(4 * time.Minute); !repository.retryAt.Equal(want) {
		t.Errorf("retryAt = %s, want %s", repository.retryAt, want)
	}
}

type repositoryStub struct {
	claimed      Job
	claimErr     error
	completedID  string
	failState    State
	retryAt      time.Time
	failedReason string
}

func (repository *repositoryStub) Claim(context.Context, string, time.Time, time.Duration) (Job, error) {
	return repository.claimed, repository.claimErr
}

func (repository *repositoryStub) Complete(_ context.Context, id, _ string, _ time.Time) error {
	repository.completedID = id
	return nil
}

func (repository *repositoryStub) Fail(_ context.Context, _, _ string, _, retryAt time.Time, failure string) (State, error) {
	repository.retryAt = retryAt
	repository.failedReason = failure
	return repository.failState, nil
}
