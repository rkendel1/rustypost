package scheduler

import (
	"context"
	"sync"

	"flux/internal/automation/jobs"
)

// Engine coordinates queued jobs, dependency ordering, and runtime dispatch.
type Engine struct {
	queue  *jobs.Queue
	runner *jobs.Runner
	events *jobs.Publisher

	mu          sync.Mutex
	pending     map[string]jobs.Job
	dependsOn   map[string][]string
	completions map[string]jobs.Status
}

func NewEngine(queue *jobs.Queue, runner *jobs.Runner, events *jobs.Publisher) *Engine {
	return &Engine{
		queue:       queue,
		runner:      runner,
		events:      events,
		pending:     map[string]jobs.Job{},
		dependsOn:   map[string][]string{},
		completions: map[string]jobs.Status{},
	}
}

func (e *Engine) Start(ctx context.Context, workers int) {
	if e.runner != nil {
		e.runner.Start(ctx, workers)
	}
	if e.events == nil {
		return
	}
	ch, _ := e.events.Subscribe(64)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case evt, ok := <-ch:
				if !ok {
					return
				}
				e.handleEvent(evt)
			}
		}
	}()
}

func (e *Engine) Submit(job jobs.Job, dependsOn ...string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(dependsOn) == 0 || e.dependenciesComplete(dependsOn) {
		e.queue.Enqueue(job)
		return
	}
	e.pending[job.ID] = job
	e.dependsOn[job.ID] = append([]string(nil), dependsOn...)
}

func (e *Engine) handleEvent(evt jobs.Event) {
	if evt.Type != jobs.EventCompleted && evt.Type != jobs.EventFailed && evt.Type != jobs.EventCancelled {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.completions[evt.Job.ID] = evt.Job.Status

	for id, job := range e.pending {
		deps := e.dependsOn[id]
		if e.dependenciesComplete(deps) {
			e.queue.Enqueue(job)
			delete(e.pending, id)
			delete(e.dependsOn, id)
		}
	}
}

func (e *Engine) dependenciesComplete(deps []string) bool {
	for _, dep := range deps {
		if e.completions[dep] != jobs.StatusCompleted {
			return false
		}
	}
	return true
}
