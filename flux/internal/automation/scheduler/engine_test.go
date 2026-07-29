package scheduler

import (
	"context"
	"testing"
	"time"

	"flux/internal/automation/jobs"
)

func TestEngineHonorsDependencies(t *testing.T) {
	q := jobs.NewQueue(8)
	pub := jobs.NewPublisher()
	runner := jobs.NewRunner(q, pub)

	runner.Register(jobs.TypeRepositoryScan, func(ctx context.Context, job jobs.Job, progress func(int)) (any, error) {
		return job.ID, nil
	})

	eng := NewEngine(q, runner, pub)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	eng.Start(ctx, 1)

	eng.Submit(jobs.Job{ID: "a", Type: jobs.TypeRepositoryScan})
	eng.Submit(jobs.Job{ID: "b", Type: jobs.TypeRepositoryScan}, "a")

	waitForStatus(t, q, "a", jobs.StatusCompleted, time.Second)
	waitForStatus(t, q, "b", jobs.StatusCompleted, time.Second)

	ja, ok := q.Get("a")
	if !ok || ja.Status != jobs.StatusCompleted {
		t.Fatalf("job a not completed: %#v", ja)
	}
	jb, ok := q.Get("b")
	if !ok || jb.Status != jobs.StatusCompleted {
		t.Fatalf("job b not completed: %#v", jb)
	}
}

func waitForStatus(t *testing.T, q *jobs.Queue, id string, status jobs.Status, timeout time.Duration) {
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
