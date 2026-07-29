package pipeline

import (
	"context"
	"errors"
	"testing"

	"flux/internal/automation/events"
)

type maskingRedactor struct{}

func (maskingRedactor) Mask(s string) string { return "[REDACTED]" }

func TestEnginePublishesRedactedErrors(t *testing.T) {
	bus := events.NewBus()
	e := NewEngine(bus)
	e.SetRedactor(maskingRedactor{})

	ch, unsub := bus.Subscribe(Topic, 16)
	defer unsub()

	failing := &testStep{
		id:    "fail",
		name:  "Fail",
		runFn: func() error { return errors.New("token=ghp_realSecret leaked") },
	}
	_, err := e.Run(context.Background(), Pipeline{
		Name:  "fails",
		Steps: []Step{failing},
	}, Metadata{ProjectID: "proj_1"})
	if err == nil {
		t.Fatal("expected pipeline error")
	}

	found := false
	for {
		select {
		case evt := <-ch:
			payload, ok := evt.Payload["error"].(string)
			if !ok || payload == "" {
				continue
			}
			found = true
			if payload != "[REDACTED]" {
				t.Fatalf("expected redacted error in event payload, got %q", payload)
			}
			if evt.ProjectID != "proj_1" {
				t.Errorf("expected ProjectID to propagate onto the event, got %q", evt.ProjectID)
			}
		default:
			if !found {
				t.Fatal("expected at least one event with a redacted error payload")
			}
			return
		}
	}
}
