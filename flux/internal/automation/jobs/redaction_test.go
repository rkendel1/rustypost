package jobs

import (
	"context"
	"errors"
	"testing"
	"time"
)

type maskingRedactor struct{}

func (maskingRedactor) Mask(s string) string { return "[REDACTED]" }

func TestRunnerRedactsJobErrorBeforePublishing(t *testing.T) {
	q := NewQueue(4)
	pub := NewPublisher()
	r := NewRunner(q, pub)
	r.SetRedactor(maskingRedactor{})
	r.Register(TypeRepositoryScan, func(ctx context.Context, job Job, progress func(int)) (any, error) {
		return nil, errors.New("ghp_realSecretToken1234 leaked in error")
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.Start(ctx)

	events, unsub := pub.Subscribe(8)
	defer unsub()

	q.Enqueue(Job{ID: "job-redact", Type: TypeRepositoryScan, ProjectID: "proj_1"})
	deadline := time.After(2 * time.Second)
	for {
		select {
		case evt := <-events:
			if evt.Type == EventFailed {
				if evt.Job.Error != "[REDACTED]" {
					t.Fatalf("expected redacted error, got %q", evt.Job.Error)
				}
				// The queue's own stored copy must be redacted too, not
				// just the published event.
				stored, ok := q.Get("job-redact")
				if !ok || stored.Error != "[REDACTED]" {
					t.Fatalf("expected queue-stored job error to be redacted, got %+v", stored)
				}
				return
			}
		case <-deadline:
			t.Fatalf("timed out waiting for failure event")
		}
	}
}
