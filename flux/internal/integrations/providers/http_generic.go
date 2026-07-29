package providers

import (
	"context"

	"flux/internal/integrations"
	"flux/internal/vault"
)

// HTTPGenericProvider is the foundation for arbitrary HTTP API integrations
// that authenticate with a single bearer/API-key credential. It requires a
// credential to be configured but does not yet make a live validation call
// (there's no fixed endpoint to call for a "generic" API) — that becomes
// meaningful once a project configures a base URL and health-check path, a
// future capability.
type HTTPGenericProvider struct{}

func NewHTTPGenericProvider() *HTTPGenericProvider { return &HTTPGenericProvider{} }

func (p *HTTPGenericProvider) ID() integrations.ProviderID { return integrations.ProviderHTTPGeneric }
func (p *HTTPGenericProvider) DisplayName() string         { return "Generic HTTP API" }

func (p *HTTPGenericProvider) CredentialRequirements() []integrations.CredentialRequirement {
	return []integrations.CredentialRequirement{
		{Purpose: vault.SecretPurposeIntegrationSync, Description: "API key or bearer token used to authenticate requests"},
	}
}

func (p *HTTPGenericProvider) Validate(ctx context.Context, in integrations.Integration, resolver vault.SecretResolver) (integrations.IntegrationHealth, error) {
	if in.Credential.SecretID == "" {
		return integrations.IntegrationHealth{Status: integrations.IntegrationStatusRequiresSetup, Detail: "no credential configured"}, nil
	}
	return integrations.IntegrationHealth{Status: integrations.IntegrationStatusConnected}, nil
}
