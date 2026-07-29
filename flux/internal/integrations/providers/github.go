// Package providers implements IntegrationProvider for Reqit's initial
// provider set: GitHub, Local Git, and a generic HTTP API. Future providers
// (Stripe, QuickBooks, Salesforce, ...) register the same way without
// requiring changes to internal/integrations itself.
package providers

import (
	"context"

	githubsvc "flux/internal/github"
	"flux/internal/integrations"
	"flux/internal/vault"
)

// GitHubProvider validates a GitHub Integration by resolving its credential
// and making a live "who am I" call — it never returns the resolved value.
type GitHubProvider struct{}

func NewGitHubProvider() *GitHubProvider { return &GitHubProvider{} }

func (p *GitHubProvider) ID() integrations.ProviderID { return integrations.ProviderGitHub }
func (p *GitHubProvider) DisplayName() string         { return "GitHub" }

func (p *GitHubProvider) CredentialRequirements() []integrations.CredentialRequirement {
	return []integrations.CredentialRequirement{
		{Purpose: vault.SecretPurposeGitHubAPI, Description: "Personal access token used to call the GitHub API"},
	}
}

func (p *GitHubProvider) Validate(ctx context.Context, in integrations.Integration, resolver vault.SecretResolver) (integrations.IntegrationHealth, error) {
	if in.Credential.SecretID == "" {
		return integrations.IntegrationHealth{Status: integrations.IntegrationStatusRequiresSetup, Detail: "no credential configured"}, nil
	}
	client := githubsvc.NewClient(&resolvingTokenProvider{ctx: ctx, resolver: resolver, ref: in.Credential})
	if _, err := client.GetViewer(ctx); err != nil {
		return integrations.IntegrationHealth{Status: integrations.IntegrationStatusError, Detail: err.Error()}, nil
	}
	return integrations.IntegrationHealth{Status: integrations.IntegrationStatusConnected}, nil
}

// resolvingTokenProvider adapts a vault.SecretResolver to
// github.TokenProvider, so github.Client can authenticate without this
// package (or github.Client) ever needing to know how the token is stored.
type resolvingTokenProvider struct {
	ctx      context.Context
	resolver vault.SecretResolver
	ref      vault.SecretReference
}

func (r *resolvingTokenProvider) LoadToken() (string, error) {
	return r.resolver.Resolve(r.ctx, r.ref, vault.SecretPurposeGitHubAPI)
}
