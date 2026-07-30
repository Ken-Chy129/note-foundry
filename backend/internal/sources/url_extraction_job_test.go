package sources

import (
	"context"
	"testing"
	"time"

	"github.com/Ken-Chy129/note-foundry/backend/internal/jobs"
)

func TestURLExtractionSchedulerEnqueuesIdempotentJob(t *testing.T) {
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	enqueuer := &sourceJobEnqueuerStub{}
	scheduler := NewURLExtractionScheduler(enqueuer, func() string { return "22222222-2222-4222-8222-222222222222" }, func() time.Time { return now })

	if err := scheduler.EnsureURLExtraction(context.Background(), "11111111-1111-4111-8111-111111111111", now); err != nil {
		t.Fatalf("EnsureURLExtraction() error = %v", err)
	}
	if enqueuer.job.Kind != JobKindExtractURL || enqueuer.job.IdempotencyKey != "source:11111111-1111-4111-8111-111111111111:extract-url:2026-07-31T12:00:00Z" {
		t.Fatalf("enqueued Job = %+v", enqueuer.job)
	}
	payload, err := DecodeURLExtractionPayload(enqueuer.job)
	if err != nil || payload.SourceID != "11111111-1111-4111-8111-111111111111" {
		t.Fatalf("DecodeURLExtractionPayload() = %+v, %v", payload, err)
	}
}

type sourceJobEnqueuerStub struct{ job jobs.Job }

func (enqueuer *sourceJobEnqueuerStub) Enqueue(_ context.Context, job jobs.Job) (jobs.Job, bool, error) {
	enqueuer.job = job
	return job, true, nil
}
