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
	time.Sleep(250 * time.Millisecond)

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

	q.Enqueue(Job{ID: "cancel-job", Type: TypeRepositoryScan})
	time.Sleep(150 * time.Millisecond)
	if ok := r.Cancel("cancel-job"); !ok {
		t.Fatalf("expected cancel to succeed")
	}
	time.Sleep(150 * time.Millisecond)

	job, ok := q.Get("cancel-job")
	if !ok {
		t.Fatalf("job missing")
	}
	if job.Status != StatusCancelled {
		t.Fatalf("expected cancelled, got %s", job.Status)
	}
}
