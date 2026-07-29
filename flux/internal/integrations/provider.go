package integrations

import (
	"context"

	"flux/internal/vault"
)

// CredentialRequirement describes one credential an IntegrationProvider
// needs in order to operate, and what it will be used for (feeding
// vault's purpose-aware resolution).
type CredentialRequirement struct {
	Purpose     vault.SecretPurpose `json:"purpose"`
	Description string              `json:"description"`
}

// IntegrationHealth is the result of validating an Integration.
type IntegrationHealth struct {
	Status IntegrationStatus `json:"status"`
	Detail string            `json:"detail,omitempty"`
}

// IntegrationProvider is the contract a vendor integration implements.
// Validate is given a vault.SecretResolver rather than a raw value — only
// the provider's own backend-side validation call ever sees the resolved
// plaintext secret, and it is never returned from Validate.
type IntegrationProvider interface {
	ID() ProviderID
	DisplayName() string
	CredentialRequirements() []CredentialRequirement
	Validate(ctx context.Context, integration Integration, resolver vault.SecretResolver) (IntegrationHealth, error)
}
