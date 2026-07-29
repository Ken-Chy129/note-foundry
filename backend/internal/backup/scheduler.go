package backup

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Ken-Chy129/note-foundry/backend/internal/jobs"
)

const JobKindCreate = "backup.create"

type JobEnqueuer interface {
	Enqueue(context.Context, jobs.Job) (jobs.Job, bool, error)
}

type Scheduler struct {
	jobs       JobEnqueuer
	generateID func() string
}

func NewScheduler(enqueuer JobEnqueuer, generateID func() string) *Scheduler {
	return &Scheduler{jobs: enqueuer, generateID: generateID}
}

type CreatePayload struct {
	ScheduledAt time.Time `json:"scheduledAt"`
}

func (scheduler *Scheduler) EnsureDaily(ctx context.Context, now time.Time) (jobs.Job, bool, error) {
	utc := now.UTC()
	scheduledAt := time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
	payload, err := json.Marshal(CreatePayload{ScheduledAt: scheduledAt})
	if err != nil {
		return jobs.Job{}, false, fmt.Errorf("encode backup Job: %w", err)
	}
	job, err := jobs.New(
		scheduler.generateID(),
		JobKindCreate,
		payload,
		3,
		now,
		"backup:daily:"+scheduledAt.Format("2006-01-02"),
	)
	if err != nil {
		return jobs.Job{}, false, err
	}
	return scheduler.jobs.Enqueue(ctx, job)
}

func DecodeCreatePayload(job jobs.Job) (CreatePayload, error) {
	var payload CreatePayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return CreatePayload{}, fmt.Errorf("decode backup Job: %w", err)
	}
	if payload.ScheduledAt.IsZero() {
		return CreatePayload{}, ErrInvalidSchedule
	}
	return payload, nil
}
