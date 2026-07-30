package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Ken-Chy129/note-foundry/backend/internal/jobs"
)

const JobKindExtractURL = "source.extract.url"

type URLExtractionPayload struct {
	SourceID string `json:"sourceId"`
}

type SourceJobEnqueuer interface {
	Enqueue(context.Context, jobs.Job) (jobs.Job, bool, error)
}

type URLExtractionScheduler struct {
	jobs       SourceJobEnqueuer
	generateID func() string
	now        func() time.Time
}

func NewURLExtractionScheduler(enqueuer SourceJobEnqueuer, generateID func() string, now func() time.Time) *URLExtractionScheduler {
	return &URLExtractionScheduler{jobs: enqueuer, generateID: generateID, now: now}
}

func (scheduler *URLExtractionScheduler) EnsureURLExtraction(ctx context.Context, sourceID string, sourceVersion time.Time) error {
	payload, err := json.Marshal(URLExtractionPayload{SourceID: sourceID})
	if err != nil {
		return fmt.Errorf("encode URL extraction Job: %w", err)
	}
	job, err := jobs.New(
		scheduler.generateID(),
		JobKindExtractURL,
		payload,
		5,
		scheduler.now(),
		"source:"+sourceID+":extract-url:"+sourceVersion.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return err
	}
	if _, _, err := scheduler.jobs.Enqueue(ctx, job); err != nil {
		return fmt.Errorf("enqueue URL extraction Job: %w", err)
	}
	return nil
}

func DecodeURLExtractionPayload(job jobs.Job) (URLExtractionPayload, error) {
	var payload URLExtractionPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil || payload.SourceID == "" {
		return URLExtractionPayload{}, fmt.Errorf("decode URL extraction Job payload")
	}
	return payload, nil
}
