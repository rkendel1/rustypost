package jobs

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

type Handler func(ctx context.Context, job Job, progress func(int)) (any, error)

// Redactor masks secret-shaped substrings out of free-text fields before
// they're published or retained in history. internal/masker.Engine
// implements this.
type Redactor interface {
	Mask(string) string
}

type Runner struct {
	queue    *Queue
	events   *Publisher
	mu       sync.RWMutex
	handlers map[Type]Handler
	cancels  map[string]context.CancelFunc
	history  []Job
	redactor Redactor
}

func NewRunner(queue *Queue, events *Publisher) *Runner {
	return &Runner{
		queue:    queue,
		events:   events,
		handlers: map[Type]Handler{},
		cancels:  map[string]context.CancelFunc{},
	}
}

func (r *Runner) Register(jobType Type, h Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[jobType] = h
}

// SetRedactor installs a Redactor applied to Job.Error before a job event is
// published or the job is retained in history. Optional — if unset, error
// text passes through as-is.
func (r *Runner) SetRedactor(red Redactor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.redactor = red
}

func (r *Runner) redact(job Job) Job {
	r.mu.RLock()
	red := r.redactor
	r.mu.RUnlock()
	if red != nil && job.Error != "" {
		job.Error = red.Mask(job.Error)
	}
	return job
}

func (r *Runner) Start(ctx context.Context, workers ...int) {
	n := 1
	if len(workers) > 0 && workers[0] > 0 {
		n = workers[0]
	}
	for i := 0; i < n; i++ {
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case id := <-r.queue.Next():
					job, ok := r.queue.Get(id)
					if !ok {
						continue
					}
					r.events.Publish(Event{Type: EventQueued, Job: job})
					r.runOne(ctx, job)
				}
			}
		}()
	}
}

func (r *Runner) Cancel(jobID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	cancel, ok := r.cancels[jobID]
	if !ok {
		return false
	}
	cancel()
	return true
}

func (r *Runner) History() []Job {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Job, len(r.history))
	copy(out, r.history)
	return out
}

func (r *Runner) appendHistory(job Job) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.history = append(r.history, job)
}

func (r *Runner) runOne(ctx context.Context, job Job) {
	r.mu.RLock()
	handler, ok := r.handlers[job.Type]
	r.mu.RUnlock()
	if !ok {
		job.Status = StatusFailed
		job.Error = fmt.Sprintf("no handler registered for %s", job.Type)
		job.FinishedAt = time.Now().UTC()
		job = r.redact(job)
		r.queue.Update(job)
		r.events.Publish(Event{Type: EventFailed, Job: job})
		r.appendHistory(job)
		return
	}
	runCtx, cancel := context.WithCancel(ctx)
	r.mu.Lock()
	r.cancels[job.ID] = cancel
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		delete(r.cancels, job.ID)
		r.mu.Unlock()
		cancel()
	}()

	job.Status = StatusRunning
	job.StartedAt = time.Now().UTC()
	r.queue.Update(job)
	r.events.Publish(Event{Type: EventRunning, Job: job})

	progress := func(v int) {
		if v < 0 {
			v = 0
		}
		if v > 100 {
			v = 100
		}
		job.Progress = v
		r.queue.Update(job)
		r.events.Publish(Event{Type: EventProgress, Job: job})
	}

	var result any
	var err error
	if job.MaxRetries < 0 {
		job.MaxRetries = 0
	}
	maxAttempts := job.MaxRetries + 1
	for {
		job.Attempts++
		result, err = handler(runCtx, job, progress)
		if err == nil {
			break
		}
		if errors.Is(err, context.Canceled) || errors.Is(runCtx.Err(), context.Canceled) {
			job.Status = StatusCancelled
			job.Error = context.Canceled.Error()
			job.FinishedAt = time.Now().UTC()
			job = r.redact(job)
			r.queue.Update(job)
			r.events.Publish(Event{Type: EventCancelled, Job: job})
			r.appendHistory(job)
			return
		}
		if job.Attempts >= maxAttempts {
			job.Status = StatusFailed
			job.Error = err.Error()
			job.FinishedAt = time.Now().UTC()
			job = r.redact(job)
			r.queue.Update(job)
			r.events.Publish(Event{Type: EventFailed, Job: job})
			r.appendHistory(job)
			return
		}
		r.queue.Update(job)
		r.events.Publish(Event{Type: EventRetried, Job: job})
	}

	job.Result = result
	job.Progress = 100
	job.Status = StatusCompleted
	job.FinishedAt = time.Now().UTC()
	r.queue.Update(job)
	r.events.Publish(Event{Type: EventCompleted, Job: job})
	r.appendHistory(job)
}
