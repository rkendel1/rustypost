// Package capabilities implements the project capability registration
// boundary: a way for the frontend to render navigation and gating based on
// what's actually available for a project, instead of a hardcoded list of
// tool sections. This is also the seam a future BackendVoid compiler
// capability plugs into — internal/projects must never import this package
// or any capability implementation; capabilities depend on the Project
// contracts, not the other way around.
package capabilities

import (
	"context"

	"flux/internal/projects"
)

// CapabilityID identifies a registered capability.
type CapabilityID string

// Availability reports whether a capability can currently be used for a
// given project.
type Availability string

const (
	CapabilityAvailable     Availability = "available"
	CapabilityUnavailable   Availability = "unavailable"
	CapabilityComingSoon    Availability = "coming_soon"
	CapabilityRequiresSetup Availability = "requires_setup"
)

// ProjectCapability is the contract every capability (present or future)
// implements. Availability must reflect real preconditions — a capability
// with no meaningful gating logic should not be registered as
// "available" unconditionally just to have an entry.
type ProjectCapability interface {
	ID() CapabilityID
	Name() string
	Description() string
	Availability(ctx context.Context, project projects.Project) Availability
}

// Registry holds the capabilities registered for this running app instance.
// It is populated once at startup (see app.go) and only read afterward, so
// it does not need its own locking beyond what a simple slice provides.
type Registry struct {
	caps []ProjectCapability
}

// NewRegistry constructs an empty capability registry.
func NewRegistry() *Registry { return &Registry{} }

// Register adds a capability. Order of registration is preserved in List/Snapshot.
func (r *Registry) Register(c ProjectCapability) {
	r.caps = append(r.caps, c)
}

// List returns all registered capabilities.
func (r *Registry) List() []ProjectCapability {
	out := make([]ProjectCapability, len(r.caps))
	copy(out, r.caps)
	return out
}

// Snapshot is the frontend-safe view of one capability's current state for
// a project — this is what a Wails binding returns, never the interface
// itself.
type Snapshot struct {
	ID           CapabilityID `json:"id"`
	Name         string       `json:"name"`
	Description  string       `json:"description"`
	Availability Availability `json:"availability"`
}

// Snapshot evaluates Availability for every registered capability against
// the given project.
func (r *Registry) Snapshot(ctx context.Context, project projects.Project) []Snapshot {
	out := make([]Snapshot, 0, len(r.caps))
	for _, c := range r.caps {
		out = append(out, Snapshot{
			ID:           c.ID(),
			Name:         c.Name(),
			Description:  c.Description(),
			Availability: c.Availability(ctx, project),
		})
	}
	return out
}
