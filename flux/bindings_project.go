package main

import (
	"errors"
	"path/filepath"
	"strings"

	"flux/internal/audit"
	"flux/internal/capabilities"
	"flux/internal/projects"
)

// --- Projects ---
//
// Project is the new root aggregate replacing Workspace as the primary
// domain object. These bindings exist alongside the existing Workspace
// bindings (bindings_*.go / app.go's GetWorkspaces, CreateWorkspace, ...)
// during the migration: workspaces.Store remains fully functional and is
// what the current frontend still calls, while these expose the same
// capability through the new Project model for forward compatibility.

func (a *App) ListProjects() ([]projects.ProjectSummary, error) {
	if a.projects == nil {
		return nil, errors.New("projects not initialised")
	}
	return a.projects.List(a.ctx)
}

func (a *App) CreateProject(name, description, rootDir string) (*projects.Project, error) {
	if a.projects == nil {
		return nil, errors.New("projects not initialised")
	}
	p, err := a.projects.Create(a.ctx, projects.CreateProjectInput{
		Name:        name,
		Description: description,
		RootDir:     rootDir,
	})
	if err != nil {
		return nil, err
	}
	if a.audit != nil {
		_ = a.audit.Log("user", audit.ActionCreate, "project", p.ID, "", map[string]string{"name": name})
	}
	return p, nil
}

func (a *App) OpenProject(projectID string) (*projects.Project, error) {
	if a.projects == nil {
		return nil, errors.New("projects not initialised")
	}
	p, err := a.projects.Open(a.ctx, projectID)
	if err != nil {
		return nil, err
	}
	a.mountProject(p)
	return p, nil
}

// GetActiveProject returns the project mounted at startup (or by the last
// OpenProject call), or nil if none is active yet — e.g. a brand new
// install with no projects created or migrated.
func (a *App) GetActiveProject() (*projects.Project, error) {
	if a.projects == nil {
		return nil, errors.New("projects not initialised")
	}
	id, ok := projects.ActiveProjectID(a.projects)
	if !ok {
		return nil, nil
	}
	return a.projects.Open(a.ctx, id)
}

func (a *App) UpdateProject(projectID string, name, description *string) (*projects.Project, error) {
	if a.projects == nil {
		return nil, errors.New("projects not initialised")
	}
	return a.projects.Update(a.ctx, projectID, projects.UpdateProjectInput{
		Name:        name,
		Description: description,
	})
}

func (a *App) ArchiveProject(projectID string) error {
	if a.projects == nil {
		return errors.New("projects not initialised")
	}
	if err := a.projects.Archive(a.ctx, projectID); err != nil {
		return err
	}
	if a.audit != nil {
		_ = a.audit.Log("user", audit.ActionDelete, "project", projectID, "", nil)
	}
	return nil
}

func (a *App) AddProjectSource(projectID, name string, kind projects.SourceType, path, url, branch string) (*projects.ProjectSource, error) {
	if a.projects == nil {
		return nil, errors.New("projects not initialised")
	}
	return a.projects.AddSource(a.ctx, projectID, projects.AddSourceInput{
		Name:   name,
		Kind:   kind,
		Path:   path,
		URL:    url,
		Branch: branch,
	})
}

func (a *App) RemoveProjectSource(projectID, sourceID string) error {
	if a.projects == nil {
		return errors.New("projects not initialised")
	}
	return a.projects.RemoveSource(a.ctx, projectID, sourceID)
}

// GetProjectCapabilities reports which capabilities (API Intelligence,
// GitHub, and future ones as they register) are available for a project,
// so the frontend can render navigation/gating from real state instead of a
// hardcoded list.
func (a *App) GetProjectCapabilities(projectID string) ([]capabilities.Snapshot, error) {
	if a.projects == nil || a.capabilities == nil {
		return nil, errors.New("projects not initialised")
	}
	p, err := a.projects.Open(a.ctx, projectID)
	if err != nil {
		return nil, err
	}
	return a.capabilities.Snapshot(a.ctx, *p), nil
}

// EnsureProjectForPath finds an existing Project rooted at path, or creates
// one (with a single source wrapping it) if none exists yet, returning the
// Project so callers can pick a source ID from it. This lets flows that
// still collect a raw filesystem path (e.g. the GitHub repo-ingest panel,
// ahead of a full Project/Source picker UI) operate through the Project
// model rather than a bare path.
func (a *App) EnsureProjectForPath(path, name string) (*projects.Project, error) {
	if a.projects == nil {
		return nil, errors.New("projects not initialised")
	}
	absPath, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return nil, err
	}
	summaries, err := a.projects.List(a.ctx)
	if err != nil {
		return nil, err
	}
	for _, s := range summaries {
		if s.RootDir == absPath {
			return a.projects.Open(a.ctx, s.ID)
		}
	}
	displayName := strings.TrimSpace(name)
	if displayName == "" {
		displayName = filepath.Base(absPath)
	}
	p, err := a.projects.Create(a.ctx, projects.CreateProjectInput{Name: displayName, RootDir: absPath})
	if err != nil {
		return nil, err
	}
	if _, err := a.projects.AddSource(a.ctx, p.ID, projects.AddSourceInput{
		Name: displayName,
		Kind: projects.DetectSourceKind(absPath),
		Path: ".",
	}); err != nil {
		return nil, err
	}
	return a.projects.Open(a.ctx, p.ID)
}
