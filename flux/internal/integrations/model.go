// Package integrations implements the Integration Registry: configured
// vendor capabilities (GitHub, Local Git, generic HTTP APIs, and future
// providers) that reference credentials through vault.SecretReference
// rather than holding them directly. An Integration is a configured
// capability; a secret is a credential — the two are deliberately separate
// so a project can reuse an application-scoped credential across many
// integrations without duplicating it.
package integrations

import "flux/internal/vault"

// ProviderID identifies which vendor/system an Integration configures.
type ProviderID string

const (
	ProviderGitHub      ProviderID = "github"
	ProviderLocalGit    ProviderID = "local_git"
	ProviderHTTPGeneric ProviderID = "http_generic"
)

// IntegrationStatus is a coarse, frontend-safe health summary.
type IntegrationStatus string

const (
	IntegrationStatusConnected     IntegrationStatus = "connected"
	IntegrationStatusRequiresSetup IntegrationStatus = "requires_setup"
	IntegrationStatusError         IntegrationStatus = "error"
)

// Integration is a configured capability bound to a credential reference.
// An empty ProjectID means the integration is application-scoped (shared
// across projects); a non-empty one scopes it to a single project.
type Integration struct {
	ID         string                `json:"id"`
	ProjectID  string                `json:"projectId,omitempty"`
	Provider   ProviderID            `json:"provider"`
	Name       string                `json:"name"`
	Status     IntegrationStatus     `json:"status"`
	Credential vault.SecretReference `json:"credential"`
	CreatedAt  string                `json:"createdAt"`
	UpdatedAt  string                `json:"updatedAt"`
}
