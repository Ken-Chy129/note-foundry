package jobs

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"
)

type Repository interface {
	Claim(context.Context, string, time.Time, time.Duration, []string) (Job, error)
	Complete(context.Context, string, string, time.Time) error
	Fail(context.Context, string, string, time.Time, time.Time, string) (State, error)
}

type Handler func(context.Context, Job) error

type Runner struct {
	repository  Repository
	workerID    string
	handlers    map[string]Handler
	kinds       []string
	now         func() time.Time
	lease       time.Duration
	baseBackoff time.Duration
	maxBackoff  time.Duration
}

type RunnerConfig struct {
	Repository  Repository
	WorkerID    string
	Handlers    map[string]Handler
	Now         func() time.Time
	Lease       time.Duration
	BaseBackoff time.Duration
	MaxBackoff  time.Duration
}

type Result struct {
	Job          Job
	State        State
	HandlerError error
}

func NewRunner(config RunnerConfig) *Runner {
	now := config.Now
	if now == nil {
		now = time.Now
	}
	lease := config.Lease
	if lease <= 0 {
		lease = 15 * time.Minute
	}
	baseBackoff := config.BaseBackoff
	if baseBackoff <= 0 {
		baseBackoff = time.Minute
	}
	maxBackoff := config.MaxBackoff
	if maxBackoff <= 0 {
		maxBackoff = time.Hour
	}
	kinds := make([]string, 0, len(config.Handlers))
	for kind := range config.Handlers {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	return &Runner{
		repository:  config.Repository,
		workerID:    config.WorkerID,
		handlers:    config.Handlers,
		kinds:       kinds,
		now:         now,
		lease:       lease,
		baseBackoff: baseBackoff,
		maxBackoff:  maxBackoff,
	}
}

func (runner *Runner) RunOnce(ctx context.Context) (Result, error) {
	if len(runner.kinds) == 0 {
		return Result{}, ErrNoJob
	}
	claimedAt := runner.now()
	job, err := runner.repository.Claim(ctx, runner.workerID, claimedAt, runner.lease, runner.kinds)
	if err != nil {
		return Result{}, err
	}

	handler, ok := runner.handlers[job.Kind]
	var handlerErr error
	if !ok {
		handlerErr = fmt.Errorf("no handler registered for Job kind %q", job.Kind)
	} else {
		handlerErr = handler(ctx, job)
	}
	finishedAt := runner.now()
	if handlerErr == nil {
		if err := runner.repository.Complete(ctx, job.ID, runner.workerID, finishedAt); err != nil {
			return Result{}, err
		}
		return Result{Job: job, State: StateSucceeded}, nil
	}

	state, err := runner.repository.Fail(ctx, job.ID, runner.workerID, finishedAt, finishedAt.Add(runner.retryDelay(job.Attempts)), handlerErr.Error())
	if err != nil {
		return Result{}, errors.Join(handlerErr, err)
	}
	return Result{Job: job, State: state, HandlerError: handlerErr}, nil
}

func (runner *Runner) retryDelay(attempt int) time.Duration {
	delay := runner.baseBackoff
	for index := 1; index < attempt && delay < runner.maxBackoff; index++ {
		if delay > runner.maxBackoff/2 {
			return runner.maxBackoff
		}
		delay *= 2
	}
	if delay > runner.maxBackoff {
		return runner.maxBackoff
	}
	return delay
}
