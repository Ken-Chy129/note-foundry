package jobs

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (repository *PostgresRepository) Enqueue(ctx context.Context, job Job) (Job, bool, error) {
	var stored Job
	row := repository.pool.QueryRow(ctx, `
		INSERT INTO jobs (id, kind, payload, max_attempts, available_at, idempotency_key)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''))
		ON CONFLICT (idempotency_key) WHERE idempotency_key IS NOT NULL DO NOTHING
		RETURNING id, kind, payload, state, attempts, max_attempts, available_at,
			locked_at, COALESCE(locked_by, ''), COALESCE(last_error, ''),
			COALESCE(idempotency_key, ''), completed_at, created_at, updated_at
	`, job.ID, job.Kind, job.Payload, job.MaxAttempts, job.AvailableAt, job.IdempotencyKey)
	if err := scanJob(row, &stored); err == nil {
		return stored, true, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return Job{}, false, fmt.Errorf("enqueue Job: %w", err)
	}

	if job.IdempotencyKey == "" {
		return Job{}, false, errors.New("enqueue Job returned no row")
	}
	row = repository.pool.QueryRow(ctx, `
		SELECT id, kind, payload, state, attempts, max_attempts, available_at,
			locked_at, COALESCE(locked_by, ''), COALESCE(last_error, ''),
			COALESCE(idempotency_key, ''), completed_at, created_at, updated_at
		FROM jobs WHERE idempotency_key = $1
	`, job.IdempotencyKey)
	if err := scanJob(row, &stored); err != nil {
		return Job{}, false, fmt.Errorf("load idempotent Job: %w", err)
	}
	return stored, false, nil
}

func (repository *PostgresRepository) Claim(ctx context.Context, workerID string, now time.Time, lease time.Duration, kinds []string) (Job, error) {
	if len(kinds) == 0 {
		return Job{}, ErrNoJob
	}
	transaction, err := repository.pool.Begin(ctx)
	if err != nil {
		return Job{}, fmt.Errorf("begin Job claim: %w", err)
	}
	defer func() { _ = transaction.Rollback(context.Background()) }()

	staleBefore := now.Add(-lease)
	if _, err := transaction.Exec(ctx, `
		UPDATE jobs
		SET state = 'failed',
			last_error = COALESCE(last_error, 'worker lease expired after final attempt'),
			locked_at = NULL,
			locked_by = NULL,
			completed_at = $1,
			updated_at = $1
		WHERE kind = ANY($3::text[])
			AND state = 'running' AND locked_at < $2 AND attempts >= max_attempts
	`, now, staleBefore, kinds); err != nil {
		return Job{}, fmt.Errorf("expire exhausted Jobs: %w", err)
	}

	var job Job
	row := transaction.QueryRow(ctx, `
		WITH candidate AS (
			SELECT id
			FROM jobs
			WHERE kind = ANY($3::text[])
				AND attempts < max_attempts
				AND (
					(state = 'pending' AND available_at <= $1)
					OR (state = 'running' AND locked_at < $2)
				)
			ORDER BY available_at, created_at, id
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		UPDATE jobs AS job
		SET state = 'running',
			attempts = job.attempts + 1,
			locked_at = $1,
			locked_by = $4,
			updated_at = $1
		FROM candidate
		WHERE job.id = candidate.id
		RETURNING job.id, job.kind, job.payload, job.state, job.attempts, job.max_attempts,
			job.available_at, job.locked_at, COALESCE(job.locked_by, ''),
			COALESCE(job.last_error, ''), COALESCE(job.idempotency_key, ''),
			job.completed_at, job.created_at, job.updated_at
	`, now, staleBefore, kinds, workerID)
	if err := scanJob(row, &job); errors.Is(err, pgx.ErrNoRows) {
		return Job{}, ErrNoJob
	} else if err != nil {
		return Job{}, fmt.Errorf("claim Job: %w", err)
	}
	if err := transaction.Commit(ctx); err != nil {
		return Job{}, fmt.Errorf("commit Job claim: %w", err)
	}
	return job, nil
}

func (repository *PostgresRepository) Complete(ctx context.Context, id, workerID string, now time.Time) error {
	result, err := repository.pool.Exec(ctx, `
		UPDATE jobs
		SET state = 'succeeded', locked_at = NULL, locked_by = NULL,
			completed_at = $3, updated_at = $3
		WHERE id = $1 AND state = 'running' AND locked_by = $2
	`, id, workerID, now)
	if err != nil {
		return fmt.Errorf("complete Job: %w", err)
	}
	if result.RowsAffected() != 1 {
		return ErrJobNotOwned
	}
	return nil
}

func (repository *PostgresRepository) Fail(ctx context.Context, id, workerID string, now, retryAt time.Time, failure string) (State, error) {
	var state State
	err := repository.pool.QueryRow(ctx, `
		UPDATE jobs
		SET state = CASE WHEN attempts >= max_attempts THEN 'failed' ELSE 'pending' END,
			available_at = CASE WHEN attempts >= max_attempts THEN available_at ELSE $4 END,
			locked_at = NULL,
			locked_by = NULL,
			last_error = $5,
			completed_at = CASE WHEN attempts >= max_attempts THEN $3::timestamptz ELSE NULL::timestamptz END,
			updated_at = $3
		WHERE id = $1 AND state = 'running' AND locked_by = $2
		RETURNING state
	`, id, workerID, now, retryAt, failure).Scan(&state)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrJobNotOwned
	}
	if err != nil {
		return "", fmt.Errorf("fail Job: %w", err)
	}
	return state, nil
}

type rowScanner interface {
	Scan(...any) error
}

func scanJob(row rowScanner, job *Job) error {
	return row.Scan(
		&job.ID,
		&job.Kind,
		&job.Payload,
		&job.State,
		&job.Attempts,
		&job.MaxAttempts,
		&job.AvailableAt,
		&job.LockedAt,
		&job.LockedBy,
		&job.LastError,
		&job.IdempotencyKey,
		&job.CompletedAt,
		&job.CreatedAt,
		&job.UpdatedAt,
	)
}
