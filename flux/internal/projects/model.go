// Package projects implements Reqit's Project domain model: the durable
// container that owns a unit of work's sources, environments, integrations,
// and (eventually) applications, deployments, and runtime state. It replaces
// the repository/workspace-centric model with Project as the root aggregate.
package projects

// ProjectStatus is a summarized operational state for a Project. It is not a
// replacement for detailed health information (see health.go).
type ProjectStatus string

const (
	ProjectStatusActive    ProjectStatus = "active"
	ProjectStatusBuilding  ProjectStatus = "building"
	ProjectStatusAttention ProjectStatus = "attention"
	ProjectStatusArchived  ProjectStatus = "archived"
)

// SourceType identifies what kind of thing a ProjectSource wraps.
type SourceType string

const (
	SourceTypeGitRepository SourceType = "git_repository"
	SourceTypeLocalFolder   SourceType = "local_folder"
	SourceTypeGeneratedApp  SourceType = "generated_app"
	SourceTypeImported      SourceType = "imported"
)

// ProjectSource is one source system contributing to a Project: a git
// repository, a local folder, a generated application, or an imported
// system. A Project may own multiple sources — it does not assume one
// repository equals one application.
type ProjectSource struct {
	ID   string     `json:"id"`
	Name string     `json:"name"`
	Kind SourceType `json:"type"`
	// Path is relative to the owning Project's RootDir for local sources,
	// kept relative so the manifest stays portable if the project tree moves.
	Path string `json:"path,omitempty"`
	// URL is the remote location for git_repository / imported sources.
	URL          string `json:"url,omitempty"`
	Branch       string `json:"branch,omitempty"`
	LastSyncedAt string `json:"lastSyncedAt,omitempty"`
}

// Project is the root aggregate for all related software assets and
// lifecycle operations. Its own fields are kept intentionally small — large,
// independently evolving concerns (environments, integrations, secrets,
// automation, artifacts) are project-owned resources managed by their own
// packages and keyed by ProjectID, not embedded here.
type Project struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	Status      ProjectStatus `json:"status"`
	// RootDir is the directory that owns this project's .reqit/ manifest and
	// local state. It is not necessarily a source itself — a project's
	// sources may live elsewhere and simply be referenced from here.
	RootDir      string          `json:"-"`
	Sources      []ProjectSource `json:"sources"`
	CreatedAt    string          `json:"createdAt"`
	UpdatedAt    string          `json:"updatedAt"`
	LastOpenedAt string          `json:"-"`
}

// ProjectContext threads project identity through service operations. It is
// the direct replacement for the old, unused services.WorkspaceContext, and
// must flow through repository scanning, generation, automation, integration
// operations, and secret resolution.
type ProjectContext struct {
	ProjectID     string
	SourceID      string
	EnvironmentID string
	PipelineID    string
	JobID         string
	CorrelationID string
}
