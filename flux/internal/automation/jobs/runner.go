package jobs

import (
	"context"
	"fmt"
)

type Handler func(ctx context.Context, job Job, progress func(int)) (any, error)

type Runner struct {
	queue    *Queue
	events   *Publisher
	handlers map[Type]Handler
}

func NewRunner(queue *Queue, events *Publisher) *Runner {
	return &Runner{
		queue:    queue,
		events:   events,
		handlers: map[Type]Handler{},
	}
}

func (r *Runner) Register(jobType Type, h Handler) {
	r.handlers[jobType] = h
}

func (r *Runner) Start(ctx context.Context) {
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
				r.runOne(ctx, job)
			}
		}
	}()
}

func (r *Runner) runOne(ctx context.Context, job Job) {
	handler, ok := r.handlers[job.Type]
	if !ok {
		job.Status = StatusFailed
		job.Error = fmt.Sprintf("no handler registered for %s", job.Type)
		r.queue.Update(job)
		r.events.Publish(Event{Type: EventFailed, Job: job})
		return
	}
	job.Status = StatusRunning
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

	result, err := handler(ctx, job, progress)
	if err != nil {
		job.Status = StatusFailed
		job.Error = err.Error()
		r.queue.Update(job)
		r.events.Publish(Event{Type: EventFailed, Job: job})
		return
	}

	job.Result = result
	job.Progress = 100
	job.Status = StatusCompleted
	r.queue.Update(job)
	r.events.Publish(Event{Type: EventCompleted, Job: job})
}
