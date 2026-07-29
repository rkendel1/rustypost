package capabilities

import (
	"context"

	"flux/internal/integrations"
	"flux/internal/projects"
)

// GitHubCapability reports whether a project has a connected GitHub
// integration. It wraps internal/integrations rather than reimplementing
// any GitHub logic itself.
type GitHubCapability struct {
	integrations integrations.IntegrationService
}

// NewGitHubCapability constructs the capability adapter.
func NewGitHubCapability(svc integrations.IntegrationService) *GitHubCapability {
	return &GitHubCapability{integrations: svc}
}

func (c *GitHubCapability) ID() CapabilityID { return "github_integration" }
func (c *GitHubCapability) Name() string     { return "GitHub" }
func (c *GitHubCapability) Description() string {
	return "Repository sync and automation via GitHub"
}

func (c *GitHubCapability) Availability(ctx context.Context, project projects.Project) Availability {
	if c.integrations == nil {
		return CapabilityUnavailable
	}
	all, err := c.integrations.List(ctx, "")
	if err != nil {
		return CapabilityUnavailable
	}
	for _, in := range all {
		if in.Provider != integrations.ProviderGitHub {
			continue
		}
		// Application-scoped (ProjectID == "") integrations are visible to
		// every project; project-scoped ones only to their own project.
		if in.ProjectID != "" && in.ProjectID != project.ID {
			continue
		}
		if in.Status == integrations.IntegrationStatusConnected {
			return CapabilityAvailable
		}
	}
	return CapabilityRequiresSetup
}
