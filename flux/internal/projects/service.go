package projects

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"flux/internal/security"
)

// ProjectSummary is the lightweight view returned by List, cheap enough to
// compute for every registered project without opening each one fully.
type ProjectSummary struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Description  string        `json:"description,omitempty"`
	Status       ProjectStatus `json:"status"`
	RootDir      string        `json:"rootDir"`
	SourceCount  int           `json:"sourceCount"`
	CreatedAt    string        `json:"createdAt"`
	LastOpenedAt string        `json:"lastOpenedAt"`
}

// CreateProjectInput describes a new Project to create.
type CreateProjectInput struct {
	Name        string
	Description string
	// RootDir is the directory that will own this project's .reqit/
	// manifest and local state.
	RootDir string
}

// UpdateProjectInput describes a partial update. Nil fields are left
// unchanged.
type UpdateProjectInput struct {
	Name        *string
	Description *string
	Status      *ProjectStatus
}

// AddSourceInput describes a new ProjectSource to attach to a Project.
type AddSourceInput struct {
	Name   string
	Kind   SourceType
	Path   string
	URL    string
	Branch string
}

var (
	ErrNameRequired    = errors.New("project name is required")
	ErrRootDirRequired = errors.New("project root directory is required")
	ErrSourceNotFound  = errors.New("project source not found")
)

// ProjectService owns Project lifecycle: creation, retrieval, update,
// archival, and source management. Repository services must not create or
// own projects directly — they operate against a Project + ProjectSource
// supplied by this service.
type ProjectService interface {
	Create(ctx context.Context, input CreateProjectInput) (*Project, error)
	Open(ctx context.Context, projectID string) (*Project, error)
	List(ctx context.Context) ([]ProjectSummary, error)
	Update(ctx context.Context, projectID string, input UpdateProjectInput) (*Project, error)
	Archive(ctx context.Context, projectID string) error
	AddSource(ctx context.Context, projectID string, input AddSourceInput) (*ProjectSource, error)
	RemoveSource(ctx context.Context, projectID string, sourceID string) error
}

type service struct {
	reg *registry
}

// NewService constructs the default ProjectService. baseDir is where the
// machine-wide project registry index (projects.json — which project IDs
// map to which root directories) is stored; callers typically pass the
// app's data directory (see storage.AppDir), matching the convention other
// subsystems in app.go already use (crypto.New(dataDir), telemetry.New(dataDir), ...).
func NewService(baseDir string) ProjectService {
	return &service{reg: newRegistry(baseDir)}
}

// ActiveProjectID returns the most recently active project's ID, for
// bootstrapping which project to mount at app startup. It is deliberately
// not part of the ProjectService interface: "the active project" is an
// app.go bootstrapping concern today (mirroring the old single
// active-workspace model), not a durable domain concept — a future
// multi-project Developer OS may have several projects open at once.
func ActiveProjectID(svc ProjectService) (string, bool) {
	concrete, ok := svc.(*service)
	if !ok {
		return "", false
	}
	id, err := concrete.reg.activeID()
	if err != nil || id == "" {
		return "", false
	}
	return id, true
}

func now() string { return time.Now().UTC().Format(time.RFC3339) }

func (s *service) Create(ctx context.Context, input CreateProjectInput) (*Project, error) {
	if input.Name == "" {
		return nil, ErrNameRequired
	}
	if input.RootDir == "" {
		return nil, ErrRootDirRequired
	}
	return s.createWithID(uuid.NewString(), input)
}

// createWithID is used by Create (fresh random ID) and by migration.go
// (a deterministic ID reused from the legacy workspace, so re-running
// migration finds an existing entry instead of creating a duplicate).
func (s *service) createWithID(id string, input CreateProjectInput) (*Project, error) {
	if err := EnsureLayout(input.RootDir); err != nil {
		return nil, err
	}
	ts := now()
	m := Manifest{
		Version:     ManifestSchemaVersion,
		ID:          id,
		Name:        input.Name,
		Description: input.Description,
		Status:      ProjectStatusActive,
		Sources:     []ProjectSource{},
		CreatedAt:   ts,
		UpdatedAt:   ts,
	}
	if err := WriteManifest(input.RootDir, m); err != nil {
		return nil, err
	}
	if err := s.reg.upsert(id, input.RootDir); err != nil {
		return nil, err
	}
	return manifestToProject(m, input.RootDir, ts), nil
}

func (s *service) Open(ctx context.Context, projectID string) (*Project, error) {
	entry, err := s.reg.get(projectID)
	if err != nil {
		return nil, err
	}
	m, err := ReadManifest(entry.RootDir)
	if err != nil {
		return nil, err
	}
	if m.ID == "" {
		return nil, ErrProjectNotFound
	}
	_ = s.reg.touch(projectID)
	return manifestToProject(m, entry.RootDir, entry.LastOpenedAt), nil
}

func (s *service) List(ctx context.Context) ([]ProjectSummary, error) {
	entries, err := s.reg.list()
	if err != nil {
		return nil, err
	}
	out := make([]ProjectSummary, 0, len(entries))
	for _, e := range entries {
		m, err := ReadManifest(e.RootDir)
		if err != nil || m.ID == "" {
			continue // skip unreadable/unmigrated entries rather than failing the whole list
		}
		status := m.Status
		if status == "" {
			status = ProjectStatusActive
		}
		out = append(out, ProjectSummary{
			ID:           m.ID,
			Name:         m.Name,
			Description:  m.Description,
			Status:       status,
			RootDir:      e.RootDir,
			SourceCount:  len(m.Sources),
			CreatedAt:    m.CreatedAt,
			LastOpenedAt: e.LastOpenedAt,
		})
	}
	return out, nil
}

func (s *service) Update(ctx context.Context, projectID string, input UpdateProjectInput) (*Project, error) {
	entry, err := s.reg.get(projectID)
	if err != nil {
		return nil, err
	}
	m, err := ReadManifest(entry.RootDir)
	if err != nil {
		return nil, err
	}
	if m.ID == "" {
		return nil, ErrProjectNotFound
	}
	if input.Name != nil {
		m.Name = *input.Name
	}
	if input.Description != nil {
		m.Description = *input.Description
	}
	if input.Status != nil {
		m.Status = *input.Status
	}
	m.UpdatedAt = now()
	if err := WriteManifest(entry.RootDir, m); err != nil {
		return nil, err
	}
	return manifestToProject(m, entry.RootDir, entry.LastOpenedAt), nil
}

func (s *service) Archive(ctx context.Context, projectID string) error {
	status := ProjectStatusArchived
	_, err := s.Update(ctx, projectID, UpdateProjectInput{Status: &status})
	return err
}

func (s *service) AddSource(ctx context.Context, projectID string, input AddSourceInput) (*ProjectSource, error) {
	entry, err := s.reg.get(projectID)
	if err != nil {
		return nil, err
	}
	m, err := ReadManifest(entry.RootDir)
	if err != nil {
		return nil, err
	}
	if m.ID == "" {
		return nil, ErrProjectNotFound
	}
	if input.Kind == SourceTypeLocalFolder && input.Path != "" {
		if err := security.ValidatePathWithinDir(entry.RootDir, filepath.Join(entry.RootDir, input.Path)); err != nil {
			return nil, err
		}
	}
	src := ProjectSource{
		ID:     uuid.NewString(),
		Name:   input.Name,
		Kind:   input.Kind,
		Path:   input.Path,
		URL:    input.URL,
		Branch: input.Branch,
	}
	m.Sources = append(m.Sources, src)
	m.UpdatedAt = now()
	if err := WriteManifest(entry.RootDir, m); err != nil {
		return nil, err
	}
	return &src, nil
}

func (s *service) RemoveSource(ctx context.Context, projectID string, sourceID string) error {
	entry, err := s.reg.get(projectID)
	if err != nil {
		return err
	}
	m, err := ReadManifest(entry.RootDir)
	if err != nil {
		return err
	}
	if m.ID == "" {
		return ErrProjectNotFound
	}
	for i, src := range m.Sources {
		if src.ID == sourceID {
			m.Sources = append(m.Sources[:i], m.Sources[i+1:]...)
			m.UpdatedAt = now()
			return WriteManifest(entry.RootDir, m)
		}
	}
	return ErrSourceNotFound
}

func manifestToProject(m Manifest, rootDir, lastOpenedAt string) *Project {
	sources := m.Sources
	if sources == nil {
		sources = []ProjectSource{}
	}
	status := m.Status
	if status == "" {
		status = ProjectStatusActive
	}
	return &Project{
		ID:           m.ID,
		Name:         m.Name,
		Description:  m.Description,
		Status:       status,
		RootDir:      rootDir,
		Sources:      sources,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
		LastOpenedAt: lastOpenedAt,
	}
}
