package jobs

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRunnerRetriesThenCompletes(t *testing.T) {
	q := NewQueue(2)
	pub := NewPublisher()
	r := NewRunner(q, pub)

	attempt := 0
	r.Register(TypeRepositoryScan, func(ctx context.Context, job Job, progress func(int)) (any, error) {
		attempt++
		if attempt == 1 {
			return nil, errors.New("temporary")
		}
		progress(50)
		return "ok", nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.Start(ctx)

	q.Enqueue(Job{ID: "retry-job", Type: TypeRepositoryScan, MaxRetries: 1})
	waitForStatus(t, q, "retry-job", StatusCompleted, time.Second)

	job, ok := q.Get("retry-job")
	if !ok {
		t.Fatalf("job missing")
	}
	if job.Status != StatusCompleted {
		t.Fatalf("expected completed, got %s", job.Status)
	}
	if job.Attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", job.Attempts)
	}
}

func TestRunnerCancelStopsRunningJob(t *testing.T) {
	q := NewQueue(2)
	pub := NewPublisher()
	r := NewRunner(q, pub)

	r.Register(TypeRepositoryScan, func(ctx context.Context, job Job, progress func(int)) (any, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
			return "late", nil
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.Start(ctx)
	events, unsub := pub.Subscribe(8)
	defer unsub()

	q.Enqueue(Job{ID: "cancel-job", Type: TypeRepositoryScan})
	waitForEvent(t, events, EventRunning, "cancel-job", time.Second)
	if ok := r.Cancel("cancel-job"); !ok {
		t.Fatalf("expected cancel to succeed")
	}
	waitForEvent(t, events, EventCancelled, "cancel-job", time.Second)

	job, ok := q.Get("cancel-job")
	if !ok {
		t.Fatalf("job missing")
	}
	if job.Status != StatusCancelled {
		t.Fatalf("expected cancelled, got %s", job.Status)
	}
}

func waitForStatus(t *testing.T, q *Queue, id string, status Status, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if job, ok := q.Get(id); ok && job.Status == status {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	job, _ := q.Get(id)
	t.Fatalf("timeout waiting for status %s; job=%#v", status, job)
}

func waitForEvent(t *testing.T, events <-chan Event, eventType EventType, id string, timeout time.Duration) {
	t.Helper()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case evt := <-events:
			if evt.Type == eventType && evt.Job.ID == id {
				return
			}
		case <-timer.C:
			t.Fatalf("timeout waiting for event %s for job %s", eventType, id)
		}
	}
}
