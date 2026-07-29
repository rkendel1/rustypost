package providers

import (
	"context"

	"flux/internal/integrations"
	"flux/internal/vault"
)

// LocalGitProvider represents a plain local git checkout with no remote
// credential requirement — it is always healthy, since there's nothing to
// authenticate. It exists as a foundation/extension point, not a fully
// featured provider: richer local-git validation (branch state, dirty
// working tree, etc.) is a future capability.
type LocalGitProvider struct{}

func NewLocalGitProvider() *LocalGitProvider { return &LocalGitProvider{} }

func (p *LocalGitProvider) ID() integrations.ProviderID { return integrations.ProviderLocalGit }
func (p *LocalGitProvider) DisplayName() string         { return "Local Git" }

func (p *LocalGitProvider) CredentialRequirements() []integrations.CredentialRequirement {
	return nil
}

func (p *LocalGitProvider) Validate(ctx context.Context, in integrations.Integration, resolver vault.SecretResolver) (integrations.IntegrationHealth, error) {
	return integrations.IntegrationHealth{Status: integrations.IntegrationStatusConnected}, nil
}
