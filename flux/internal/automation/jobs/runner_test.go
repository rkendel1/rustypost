package jobs

import (
	"context"
	"testing"
	"time"
)

func TestRunnerExecutesJobAndPublishesEvents(t *testing.T) {
	q := NewQueue(4)
	pub := NewPublisher()
	r := NewRunner(q, pub)
	r.Register(TypeRepositoryScan, func(ctx context.Context, job Job, progress func(int)) (any, error) {
		progress(65)
		return map[string]any{"ok": true}, nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.Start(ctx)

	events, unsub := pub.Subscribe(8)
	defer unsub()

	q.Enqueue(Job{ID: "job-123", Type: TypeRepositoryScan})
	deadline := time.After(2 * time.Second)
	var completed Job
	for {
		select {
		case evt := <-events:
			if evt.Type == EventCompleted {
				completed = evt.Job
				if completed.Status != StatusCompleted || completed.Progress != 100 {
					t.Fatalf("unexpected completed job: %#v", completed)
				}
				return
			}
		case <-deadline:
			t.Fatalf("timed out waiting for completion event")
		}
	}
}
