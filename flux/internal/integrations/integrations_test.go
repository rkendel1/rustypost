package integrations

import (
	"context"
	"testing"

	githubsvc "flux/internal/github"
	"flux/internal/vault"
	"flux/internal/vault/providers"
)

func newTestVault(t *testing.T) (vault.VaultService, vault.SecretResolver, string) {
	t.Helper()
	dir := t.TempDir()
	mem := providers.NewMemoryProvider()
	return vault.NewService(dir, mem), vault.NewResolver(dir, mem), dir
}

func TestCreateListRemoveIntegration(t *testing.T) {
	vaultSvc, resolver, _ := newTestVault(t)
	svc := NewService(t.TempDir(), resolver)
	ctx := context.Background()

	ref, err := vaultSvc.StoreSecret(ctx, vault.StoreSecretInput{
		Name: "GitHub Account", Type: vault.SecretTypeAccessToken, Value: "ghp_token",
	})
	if err != nil {
		t.Fatalf("StoreSecret: %v", err)
	}

	in, err := svc.Create(ctx, CreateIntegrationInput{
		Provider: ProviderGitHub, Name: "GitHub (default)", Credential: ref,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if in.Status != IntegrationStatusConnected {
		t.Errorf("expected connected status once a credential is set, got %s", in.Status)
	}

	list, err := svc.List(ctx, "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0].ID != in.ID {
		t.Fatalf("expected exactly the created integration, got %+v", list)
	}

	if err := svc.Remove(ctx, in.ID); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	list, err = svc.List(ctx, "")
	if err != nil {
		t.Fatalf("List after remove: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected no integrations after remove, got %d", len(list))
	}

	// Removing the Integration must never delete the underlying secret —
	// it may be shared elsewhere; secret deletion is its own explicit action.
	secrets, err := vaultSvc.ListSecrets(ctx, "")
	if err != nil {
		t.Fatalf("ListSecrets: %v", err)
	}
	if len(secrets) != 1 {
		t.Fatalf("expected the vault secret to survive Integration removal, got %d secrets", len(secrets))
	}
}

func TestCreateRequiresNameAndProvider(t *testing.T) {
	_, resolver, _ := newTestVault(t)
	svc := NewService(t.TempDir(), resolver)
	ctx := context.Background()

	if _, err := svc.Create(ctx, CreateIntegrationInput{Provider: ProviderGitHub}); err != ErrNameRequired {
		t.Errorf("expected ErrNameRequired, got %v", err)
	}
	if _, err := svc.Create(ctx, CreateIntegrationInput{Name: "n"}); err != ErrProviderRequired {
		t.Errorf("expected ErrProviderRequired, got %v", err)
	}
}

func TestCreateWithoutCredentialRequiresSetup(t *testing.T) {
	_, resolver, _ := newTestVault(t)
	svc := NewService(t.TempDir(), resolver)
	in, err := svc.Create(context.Background(), CreateIntegrationInput{Name: "n", Provider: ProviderLocalGit})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if in.Status != IntegrationStatusRequiresSetup {
		t.Errorf("expected requires_setup without a credential, got %s", in.Status)
	}
}

func TestListFiltersByProject(t *testing.T) {
	_, resolver, _ := newTestVault(t)
	svc := NewService(t.TempDir(), resolver)
	ctx := context.Background()

	if _, err := svc.Create(ctx, CreateIntegrationInput{Name: "app-wide", Provider: ProviderGitHub}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := svc.Create(ctx, CreateIntegrationInput{Name: "scoped", Provider: ProviderGitHub, ProjectID: "proj_a"}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	filtered, err := svc.List(ctx, "proj_a")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(filtered) != 1 || filtered[0].Name != "scoped" {
		t.Fatalf("expected only proj_a's integration, got %+v", filtered)
	}
}

func TestMigrateGitHubPATIsLazyAndIdempotent(t *testing.T) {
	vaultSvc, resolver, _ := newTestVault(t)
	svc := NewService(t.TempDir(), resolver)
	ctx := context.Background()

	// No PAT saved anywhere yet — migration must be a safe no-op.
	if err := MigrateGitHubPAT(ctx, "test-account", vaultSvc, svc); err != nil {
		t.Fatalf("expected no error migrating with no PAT present: %v", err)
	}
	list, err := svc.List(ctx, "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected no integration created without a legacy PAT, got %d", len(list))
	}

	// Save a legacy PAT via the pre-existing github.AuthService keychain path.
	auth := githubsvc.NewAuthService("test-account")
	if err := auth.SaveToken("ghp_legacyTokenValue"); err != nil {
		t.Skipf("skipping: OS keychain unavailable in this environment: %v", err)
	}
	t.Cleanup(func() { _ = auth.DeleteToken() })

	if err := MigrateGitHubPAT(ctx, "test-account", vaultSvc, svc); err != nil {
		t.Fatalf("MigrateGitHubPAT: %v", err)
	}
	list, err = svc.List(ctx, "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected exactly 1 migrated integration, got %d", len(list))
	}
	if list[0].Provider != ProviderGitHub || list[0].Status != IntegrationStatusConnected {
		t.Errorf("unexpected migrated integration: %+v", list[0])
	}

	// The legacy keychain entry must still be readable — migration is
	// non-destructive.
	token, err := auth.LoadToken()
	if err != nil || token != "ghp_legacyTokenValue" {
		t.Errorf("expected legacy PAT to remain intact after migration, got %q, err=%v", token, err)
	}

	// Re-running migration must not create a duplicate integration.
	if err := MigrateGitHubPAT(ctx, "test-account", vaultSvc, svc); err != nil {
		t.Fatalf("second migration: %v", err)
	}
	list, err = svc.List(ctx, "")
	if err != nil {
		t.Fatalf("List after re-migration: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected migration to remain idempotent, got %d integrations", len(list))
	}
}
