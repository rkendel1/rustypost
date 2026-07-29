package lifecycle

import (
	"context"
	"errors"
	"testing"
)

type testService struct {
	name     string
	startErr error
	started  bool
	stopped  bool
}

func (s *testService) Name() string { return s.name }
func (s *testService) Initialize(ctx context.Context) error {
	s.started = true
	return s.startErr
}
func (s *testService) Shutdown(ctx context.Context) error {
	s.stopped = true
	return nil
}

func TestManagerStartStop(t *testing.T) {
	m := NewManager()
	a := &testService{name: "a"}
	b := &testService{name: "b"}
	m.Register(a)
	m.Register(b)
	if err := m.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	if !a.started || !b.started {
		t.Fatalf("expected services to start")
	}
	if err := m.Stop(context.Background()); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if !a.stopped || !b.stopped {
		t.Fatalf("expected services to stop")
	}
}

func TestManagerRollbackOnStartFailure(t *testing.T) {
	m := NewManager()
	a := &testService{name: "a"}
	b := &testService{name: "b", startErr: errors.New("boom")}
	m.Register(a)
	m.Register(b)
	if err := m.Start(context.Background()); err == nil {
		t.Fatalf("expected error")
	}
	if !a.stopped {
		t.Fatalf("expected rollback shutdown")
	}
}
