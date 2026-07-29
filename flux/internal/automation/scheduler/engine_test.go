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

	time.Sleep(300 * time.Millisecond)

	ja, ok := q.Get("a")
	if !ok || ja.Status != jobs.StatusCompleted {
		t.Fatalf("job a not completed: %#v", ja)
	}
	jb, ok := q.Get("b")
	if !ok || jb.Status != jobs.StatusCompleted {
		t.Fatalf("job b not completed: %#v", jb)
	}
}
