package integrations

import (
	"context"
	"strings"

	githubsvc "flux/internal/github"
	"flux/internal/vault"
)

// MigrateGitHubPAT lazily imports a PAT already stored under the legacy
// "reqit-github" OS keychain entry (internal/github/auth.go) into the new
// vault + integration model. It is one-way, lazy, and non-destructive: it
// never deletes or overwrites the legacy keychain entry, so
// github.AuthService and everything that already depends on it
// (GitHubGetViewer, GitHubListRepositories, GitHubCloneRepository) keep
// working exactly as before, unaffected by whether this migration has run.
//
// Idempotent: does nothing if a github Integration for this account already
// exists, or if no legacy PAT is found.
func MigrateGitHubPAT(ctx context.Context, account string, vaultSvc vault.VaultService, integrationSvc IntegrationService) error {
	account = strings.TrimSpace(account)
	if account == "" {
		account = "default"
	}
	integrationName := "GitHub (" + account + ")"

	existing, err := integrationSvc.List(ctx, "")
	if err != nil {
		return err
	}
	for _, in := range existing {
		if in.Provider == ProviderGitHub && in.Name == integrationName {
			return nil // already migrated
		}
	}

	token, err := githubsvc.NewAuthService(account).LoadToken()
	if err != nil || strings.TrimSpace(token) == "" {
		return nil // nothing to migrate
	}

	ref, err := vaultSvc.StoreSecret(ctx, vault.StoreSecretInput{
		Name:  integrationName,
		Type:  vault.SecretTypeAccessToken,
		Scope: vault.SecretScopeApplication,
		Value: token,
	})
	if err != nil {
		return err
	}
	_, err = integrationSvc.Create(ctx, CreateIntegrationInput{
		Provider:   ProviderGitHub,
		Name:       integrationName,
		Credential: ref,
	})
	return err
}
