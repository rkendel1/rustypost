package services

import "context"

// Service defines the common lifecycle for backend capabilities.
type Service interface {
	Name() string
	Initialize(context.Context) error
	Shutdown(context.Context) error
}

// WorkspaceContext is the execution scope for automation operations.
type WorkspaceContext struct {
	WorkspaceID   string
	Repository    string
	Branch        string
	Commit        string
	Framework     string
	Languages     []string
	GeneratedDirs map[string]string
}

// Scanner is an optional capability implemented by repository scanners.
type Scanner interface {
	Scan(context.Context, WorkspaceContext) (any, error)
}

// Generator is an optional capability for artifact generation services.
type Generator interface {
	Generate(context.Context, WorkspaceContext, any) (any, error)
}

// Executor is an optional capability for runnable actions (tests, commands, etc.).
type Executor interface {
	Execute(context.Context, WorkspaceContext, any) (any, error)
}

// Validator is an optional capability for validations.
type Validator interface {
	Validate(context.Context, WorkspaceContext, any) error
}

// Reporter is an optional capability for report generation.
type Reporter interface {
	Report(context.Context, WorkspaceContext, any) (any, error)
}
