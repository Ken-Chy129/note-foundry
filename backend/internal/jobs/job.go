package jobs

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type State string

const (
	StatePending   State = "pending"
	StateRunning   State = "running"
	StateSucceeded State = "succeeded"
	StateFailed    State = "failed"
)

var (
	ErrNoJob              = errors.New("no Job is available")
	ErrJobNotOwned        = errors.New("Job is not owned by this worker")
	ErrJobIDRequired      = errors.New("Job id is required")
	ErrJobKindRequired    = errors.New("Job kind is required")
	ErrInvalidMaxAttempts = errors.New("Job max attempts must be positive")
)

type Job struct {
	ID             string
	Kind           string
	Payload        json.RawMessage
	State          State
	Attempts       int
	MaxAttempts    int
	AvailableAt    time.Time
	LockedAt       *time.Time
	LockedBy       string
	LastError      string
	IdempotencyKey string
	CompletedAt    *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func New(id, kind string, payload json.RawMessage, maxAttempts int, availableAt time.Time, idempotencyKey string) (Job, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Job{}, ErrJobIDRequired
	}
	kind = strings.TrimSpace(kind)
	if kind == "" {
		return Job{}, ErrJobKindRequired
	}
	if maxAttempts <= 0 {
		return Job{}, ErrInvalidMaxAttempts
	}
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	if !json.Valid(payload) {
		return Job{}, errors.New("Job payload must be valid JSON")
	}
	return Job{
		ID:             id,
		Kind:           kind,
		Payload:        append(json.RawMessage(nil), payload...),
		State:          StatePending,
		MaxAttempts:    maxAttempts,
		AvailableAt:    availableAt,
		IdempotencyKey: strings.TrimSpace(idempotencyKey),
	}, nil
}
