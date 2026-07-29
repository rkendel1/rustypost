package capabilities

import (
	"context"

	"flux/internal/projects"
)

// APIIntelligenceCapability wraps the existing repository scanning, OpenAPI
// generation, collections, and test-suite packages (internal/scanner,
// internal/openapi, internal/collections, internal/testbuilder) as a
// registered project capability. It doesn't reimplement any of that logic —
// it only reports whether the project currently has a source it can be run
// against.
type APIIntelligenceCapability struct{}

// NewAPIIntelligenceCapability constructs the capability adapter.
func NewAPIIntelligenceCapability() *APIIntelligenceCapability {
	return &APIIntelligenceCapability{}
}

func (c *APIIntelligenceCapability) ID() CapabilityID { return "api_intelligence" }
func (c *APIIntelligenceCapability) Name() string     { return "API Intelligence" }
func (c *APIIntelligenceCapability) Description() string {
	return "Repository scanning, OpenAPI generation, collections, and tests"
}

func (c *APIIntelligenceCapability) Availability(ctx context.Context, project projects.Project) Availability {
	for _, src := range project.Sources {
		if _, err := projects.ResolveSourcePath(project.RootDir, src); err == nil {
			return CapabilityAvailable
		}
	}
	return CapabilityRequiresSetup
}
