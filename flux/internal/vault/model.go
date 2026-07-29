// Package vault implements Reqit's Secrets & Integrations Vault: a
// local-first store for credentials that never exposes raw secret values to
// the frontend or persists them in plaintext. Services receive opaque
// SecretReferences; only a backend-only SecretResolver (resolver.go) ever
// sees a plaintext value.
package vault

// SecretType describes what kind of credential a secret holds.
type SecretType string

const (
	SecretTypeAPIKey       SecretType = "api_key"
	SecretTypeAccessToken  SecretType = "access_token"
	SecretTypeRefreshToken SecretType = "refresh_token"
	SecretTypePassword     SecretType = "password"
	SecretTypePrivateKey   SecretType = "private_key"
	SecretTypeConnection   SecretType = "connection"
	SecretTypeCustom       SecretType = "custom"
)

// SecretScope describes how broadly a secret applies.
type SecretScope string

const (
	SecretScopeApplication SecretScope = "application"
	SecretScopeProject     SecretScope = "project"
	SecretScopeEnvironment SecretScope = "environment"
)

// SecretStatus is a coarse, frontend-safe status for a secret.
type SecretStatus string

const (
	SecretStatusConfigured SecretStatus = "configured"
	SecretStatusError      SecretStatus = "error"
)

// ProviderKind identifies which credential backend holds a secret's value.
type ProviderKind string

const (
	ProviderOSKeychain    ProviderKind = "os_keychain"
	ProviderEncryptedFile ProviderKind = "encrypted_file"
)

// SecretPurpose scopes what a resolved secret value may legitimately be used
// for (see policy.go). This is the foundation for future capability-scoped
// secret access — a GitHub integration resolving a secret for
// SecretPurposeGitHubAPI is a very different trust boundary than a job
// resolving one for SecretPurposeDeployment.
type SecretPurpose string

const (
	SecretPurposeGitHubAPI       SecretPurpose = "github_api"
	SecretPurposeRepositoryClone SecretPurpose = "repository_clone"
	SecretPurposeTestExecution   SecretPurpose = "test_execution"
	SecretPurposeDeployment      SecretPurpose = "deployment"
	SecretPurposeIntegrationSync SecretPurpose = "integration_sync"
	SecretPurposeGeneric         SecretPurpose = "generic"
)

// SecretMetadata describes a secret without ever holding its value. This is
// the only representation of a secret that may reach the frontend, enter
// logs, or be embedded in a project manifest.
type SecretMetadata struct {
	ID          string       `json:"id"`
	ProjectID   string       `json:"projectId,omitempty"`
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`
	Type        SecretType   `json:"type"`
	Provider    ProviderKind `json:"provider"`
	Scope       SecretScope  `json:"scope"`
	Status      SecretStatus `json:"status"`
	CreatedAt   string       `json:"createdAt"`
	UpdatedAt   string       `json:"updatedAt"`
	LastUsedAt  string       `json:"lastUsedAt,omitempty"`
}

// SecretReference is an opaque pointer to a secret, safe to embed in a
// project manifest, an Integration's configuration, or anywhere else that
// isn't the backend-only SecretResolver — it carries no information that
// could be used to derive the secret value itself.
type SecretReference struct {
	SecretID string `json:"secretId"`
}
