package pipeline

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"flux/internal/automation/events"
)

const Topic = "automation.pipeline"

type EventType string

const (
	EventPipelineStarted   EventType = "PipelineStarted"
	EventStepStarted       EventType = "StepStarted"
	EventStepCompleted     EventType = "StepCompleted"
	EventStepRetried       EventType = "StepRetried"
	EventStepFailed        EventType = "StepFailed"
	EventPipelineCancelled EventType = "PipelineCancelled"
	EventPipelineFailed    EventType = "PipelineFailed"
	EventPipelineCompleted EventType = "PipelineCompleted"
)

// Metadata carries project context through a pipeline run, aligned 1:1 with
// projects.ProjectContext so the two can convert directly.
type Metadata struct {
	CorrelationID string
	ProjectID     string
	SourceID      string
	JobID         string
	PipelineID    string
}

type Step interface {
	ID() string
	Name() string
	Run(context.Context, *State) error
	Rollback(context.Context, *State) error
	CanRetry(error, int) bool
	Progress() int
}

type Pipeline struct {
	Name         string
	Steps        []Step
	Dependencies map[string][]string
	Inputs       map[string]any
}

type State struct {
	mu      sync.RWMutex
	Inputs  map[string]any
	Outputs map[string]any
}

func (s *State) SetOutput(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Outputs == nil {
		s.Outputs = map[string]any{}
	}
	s.Outputs[key] = value
}

func (s *State) Output(key string) (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.Outputs[key]
	return v, ok
}

// Redactor masks secret-shaped substrings out of free-text fields before
// they're published — satisfies the "secrets never in job/pipeline events"
// invariant without this package needing to know anything about vault or
// what a secret looks like. internal/masker.Engine implements this.
type Redactor interface {
	Mask(string) string
}

type Engine struct {
	bus      *events.Bus
	redactor Redactor
}

func NewEngine(bus *events.Bus) *Engine {
	if bus == nil {
		bus = events.NewBus()
	}
	return &Engine{bus: bus}
}

// SetRedactor installs a Redactor applied to error messages before they're
// published. Optional — if unset, error text is published as-is.
func (e *Engine) SetRedactor(r Redactor) {
	e.redactor = r
}

func (e *Engine) Run(ctx context.Context, p Pipeline, meta Metadata) (*State, error) {
	if p.Name == "" {
		return nil, errors.New("pipeline name is required")
	}
	if len(p.Steps) == 0 {
		return nil, errors.New("pipeline must contain at least one step")
	}
	stepsByID := map[string]Step{}
	for _, step := range p.Steps {
		if step == nil {
			return nil, errors.New("pipeline contains a nil step")
		}
		id := step.ID()
		if id == "" {
			return nil, errors.New("step id is required")
		}
		if _, exists := stepsByID[id]; exists {
			return nil, fmt.Errorf("duplicate step id: %s", id)
		}
		stepsByID[id] = step
	}
	if err := validateDependencies(stepsByID, p.Dependencies); err != nil {
		return nil, err
	}

	state := &State{Inputs: cloneMap(p.Inputs)}
	e.publish(meta, EventPipelineStarted, p.Name, "", "", 0, "")

	completed := make([]Step, 0, len(p.Steps))
	done := map[string]bool{}

	for len(done) < len(p.Steps) {
		next := nextReadyStep(p.Steps, p.Dependencies, done)
		if next == nil {
			err := errors.New("unable to resolve step dependencies")
			e.publish(meta, EventPipelineFailed, p.Name, "", "", 0, err.Error())
			rollback(ctx, completed, state)
			return nil, err
		}
		if err := ctx.Err(); err != nil {
			e.publish(meta, EventPipelineCancelled, p.Name, next.ID(), next.Name(), 0, err.Error())
			rollback(ctx, completed, state)
			return nil, err
		}

		e.publish(meta, EventStepStarted, p.Name, next.ID(), next.Name(), next.Progress(), "")

		attempt := 0
		var runErr error
		for {
			attempt++
			runErr = next.Run(ctx, state)
			if runErr == nil {
				break
			}
			if errors.Is(runErr, context.Canceled) || errors.Is(runErr, context.DeadlineExceeded) {
				e.publish(meta, EventPipelineCancelled, p.Name, next.ID(), next.Name(), next.Progress(), runErr.Error())
				rollback(ctx, completed, state)
				return nil, runErr
			}
			if !next.CanRetry(runErr, attempt) {
				break
			}
			e.publish(meta, EventStepRetried, p.Name, next.ID(), next.Name(), next.Progress(), runErr.Error())
		}
		if runErr != nil {
			e.publish(meta, EventStepFailed, p.Name, next.ID(), next.Name(), next.Progress(), runErr.Error())
			e.publish(meta, EventPipelineFailed, p.Name, next.ID(), next.Name(), next.Progress(), runErr.Error())
			rollback(ctx, completed, state)
			return nil, runErr
		}
		done[next.ID()] = true
		completed = append(completed, next)
		e.publish(meta, EventStepCompleted, p.Name, next.ID(), next.Name(), next.Progress(), "")
	}

	e.publish(meta, EventPipelineCompleted, p.Name, "", "", 100, "")
	return state, nil
}

func (e *Engine) publish(meta Metadata, typ EventType, pipelineName, stepID, stepName string, progress int, errMsg string) {
	if e.redactor != nil && errMsg != "" {
		errMsg = e.redactor.Mask(errMsg)
	}
	e.bus.Publish(Topic, events.Event{
		Name:          string(typ),
		CorrelationID: meta.CorrelationID,
		ProjectID:     meta.ProjectID,
		SourceID:      meta.SourceID,
		JobID:         meta.JobID,
		PipelineID:    meta.PipelineID,
		Timestamp:     time.Now().UTC(),
		Payload: map[string]any{
			"pipeline": pipelineName,
			"stepId":   stepID,
			"stepName": stepName,
			"progress": progress,
			"error":    errMsg,
		},
	})
}

func rollback(ctx context.Context, completed []Step, state *State) {
	for i := len(completed) - 1; i >= 0; i-- {
		_ = completed[i].Rollback(ctx, state)
	}
}

func validateDependencies(steps map[string]Step, deps map[string][]string) error {
	for stepID, stepDeps := range deps {
		if _, ok := steps[stepID]; !ok {
			return fmt.Errorf("dependency declared for unknown step %s", stepID)
		}
		for _, dep := range stepDeps {
			if _, ok := steps[dep]; !ok {
				return fmt.Errorf("step %s depends on unknown step %s", stepID, dep)
			}
		}
	}
	return nil
}

func nextReadyStep(ordered []Step, deps map[string][]string, done map[string]bool) Step {
	for _, s := range ordered {
		if done[s.ID()] {
			continue
		}
		ready := true
		for _, dep := range deps[s.ID()] {
			if !done[dep] {
				ready = false
				break
			}
		}
		if ready {
			return s
		}
	}
	return nil
}

func cloneMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
