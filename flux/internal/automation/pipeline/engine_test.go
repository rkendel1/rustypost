package pipeline

import (
	"context"
	"errors"
	"testing"

	"flux/internal/automation/events"
)

type testStep struct {
	id         string
	name       string
	progress   int
	runFn      func() error
	rollbackFn func()
	retryOnErr bool
	runs       int
	rollbacks  int
}

func (s *testStep) ID() string    { return s.id }
func (s *testStep) Name() string  { return s.name }
func (s *testStep) Progress() int { return s.progress }
func (s *testStep) Run(ctx context.Context, state *State) error {
	s.runs++
	if s.runFn != nil {
		if err := s.runFn(); err != nil {
			return err
		}
	}
	state.SetOutput(s.id, s.runs)
	return nil
}
func (s *testStep) Rollback(ctx context.Context, state *State) error {
	s.rollbacks++
	if s.rollbackFn != nil {
		s.rollbackFn()
	}
	return nil
}
func (s *testStep) CanRetry(err error, attempt int) bool {
	return s.retryOnErr && attempt < 2
}

func TestEngineRunsPipelineWithDependencies(t *testing.T) {
	bus := events.NewBus()
	e := NewEngine(bus)

	first := &testStep{id: "scan", name: "Scan", progress: 25}
	second := &testStep{id: "generate", name: "Generate", progress: 80}

	state, err := e.Run(context.Background(), Pipeline{
		Name:  "repo-setup",
		Steps: []Step{first, second},
		Dependencies: map[string][]string{
			"generate": {"scan"},
		},
	}, Metadata{PipelineID: "p-1"})
	if err != nil {
		t.Fatalf("run pipeline: %v", err)
	}
	if first.runs != 1 || second.runs != 1 {
		t.Fatalf("unexpected step runs: scan=%d generate=%d", first.runs, second.runs)
	}
	if _, ok := state.Output("scan"); !ok {
		t.Fatalf("expected scan output")
	}
}

func TestEngineRetriesAndRollsBack(t *testing.T) {
	bus := events.NewBus()
	e := NewEngine(bus)

	okStep := &testStep{id: "ok", name: "OK", progress: 50}
	failing := &testStep{
		id:         "fail",
		name:       "Fail",
		progress:   75,
		retryOnErr: true,
		runFn:      func() error { return errors.New("boom") },
	}

	_, err := e.Run(context.Background(), Pipeline{
		Name:         "fails",
		Steps:        []Step{okStep, failing},
		Dependencies: map[string][]string{"fail": {"ok"}},
	}, Metadata{})
	if err == nil {
		t.Fatalf("expected pipeline error")
	}
	if failing.runs != 2 {
		t.Fatalf("expected one retry; runs=%d", failing.runs)
	}
	if okStep.rollbacks != 1 {
		t.Fatalf("expected rollback for completed step")
	}
}
