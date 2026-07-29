package vault

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"flux/internal/vault/providers"
)

func TestStoreListDeleteSecret(t *testing.T) {
	dir := t.TempDir()
	mem := providers.NewMemoryProvider()
	svc := NewService(dir, mem)
	ctx := context.Background()

	ref, err := svc.StoreSecret(ctx, StoreSecretInput{
		Name: "GitHub Account", Type: SecretTypeAccessToken, Scope: SecretScopeApplication,
		Value: "ghp_realtoken",
	})
	if err != nil {
		t.Fatalf("StoreSecret: %v", err)
	}
	if ref.SecretID == "" {
		t.Fatal("expected a generated secret ID")
	}

	list, err := svc.ListSecrets(ctx, "")
	if err != nil {
		t.Fatalf("ListSecrets: %v", err)
	}
	if len(list) != 1 || list[0].ID != ref.SecretID {
		t.Fatalf("expected exactly the stored secret in ListSecrets, got %+v", list)
	}
	if list[0].Name != "GitHub Account" || list[0].Status != SecretStatusConfigured {
		t.Errorf("unexpected metadata: %+v", list[0])
	}

	if err := svc.DeleteSecret(ctx, ref); err != nil {
		t.Fatalf("DeleteSecret: %v", err)
	}
	list, err = svc.ListSecrets(ctx, "")
	if err != nil {
		t.Fatalf("ListSecrets after delete: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected no secrets after delete, got %d", len(list))
	}
	// the underlying credential must be gone too, not just the metadata row
	if _, err := mem.Get(ref.SecretID); err != providers.ErrNotFound {
		t.Errorf("expected credential to be deleted from the provider, got %v", err)
	}
}

func TestStoreSecretRequiresNameAndValue(t *testing.T) {
	svc := NewService(t.TempDir(), providers.NewMemoryProvider())
	ctx := context.Background()

	if _, err := svc.StoreSecret(ctx, StoreSecretInput{Value: "v"}); err != ErrNameRequired {
		t.Errorf("expected ErrNameRequired, got %v", err)
	}
	if _, err := svc.StoreSecret(ctx, StoreSecretInput{Name: "n"}); err != ErrValueRequired {
		t.Errorf("expected ErrValueRequired, got %v", err)
	}
}

func TestStoreSecretFailsWithoutBackend(t *testing.T) {
	svc := NewService(t.TempDir()) // no providers registered
	_, err := svc.StoreSecret(context.Background(), StoreSecretInput{Name: "n", Value: "v"})
	if err != ErrBackendUnavailable {
		t.Errorf("expected ErrBackendUnavailable, got %v", err)
	}
}

func TestRotateSecret(t *testing.T) {
	dir := t.TempDir()
	mem := providers.NewMemoryProvider()
	svc := NewService(dir, mem)
	ctx := context.Background()

	ref, err := svc.StoreSecret(ctx, StoreSecretInput{Name: "n", Value: "old", Type: SecretTypeAPIKey})
	if err != nil {
		t.Fatalf("StoreSecret: %v", err)
	}
	if err := svc.Rotate(ctx, ref, "new"); err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	got, err := mem.Get(ref.SecretID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "new" {
		t.Errorf("expected rotated value, got %q", got)
	}
}

func TestListSecretsFiltersByProject(t *testing.T) {
	dir := t.TempDir()
	svc := NewService(dir, providers.NewMemoryProvider())
	ctx := context.Background()

	if _, err := svc.StoreSecret(ctx, StoreSecretInput{Name: "app-wide", Value: "v1", Scope: SecretScopeApplication}); err != nil {
		t.Fatalf("StoreSecret: %v", err)
	}
	if _, err := svc.StoreSecret(ctx, StoreSecretInput{Name: "project-a", Value: "v2", ProjectID: "proj_a", Scope: SecretScopeProject}); err != nil {
		t.Fatalf("StoreSecret: %v", err)
	}

	filtered, err := svc.ListSecrets(ctx, "proj_a")
	if err != nil {
		t.Fatalf("ListSecrets: %v", err)
	}
	if len(filtered) != 1 || filtered[0].Name != "project-a" {
		t.Fatalf("expected only proj_a's secret, got %+v", filtered)
	}

	all, err := svc.ListSecrets(ctx, "")
	if err != nil {
		t.Fatalf("ListSecrets all: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected both secrets with no project filter, got %d", len(all))
	}
}

func TestSecretMetadataNeverContainsValue(t *testing.T) {
	dir := t.TempDir()
	svc := NewService(dir, providers.NewMemoryProvider())
	const secretValue = "ghp_veryRealSecretValue"
	if _, err := svc.StoreSecret(context.Background(), StoreSecretInput{Name: "n", Value: secretValue}); err != nil {
		t.Fatalf("StoreSecret: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, registryFile))
	if err != nil {
		t.Fatalf("reading registry file: %v", err)
	}
	if containsSubstringVault(string(data), secretValue) {
		t.Errorf("expected secret value to never be persisted in metadata, file contents: %s", data)
	}
}

func containsSubstringVault(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

func TestResolverResolvesAndEnforcesPurpose(t *testing.T) {
	dir := t.TempDir()
	mem := providers.NewMemoryProvider()
	svc := NewService(dir, mem)
	ctx := context.Background()

	// The resolver is constructed up front, before any secret exists — it
	// and svc are separate long-lived instances by design (that separation
	// is the security boundary keeping bindings_*.go away from resolved
	// values). It must still see secrets stored afterward through svc,
	// since in the real app a resolver isn't recreated every time a secret
	// changes.
	resolver := NewResolver(dir, mem)

	ref, err := svc.StoreSecret(ctx, StoreSecretInput{
		Name: "GitHub Account", Type: SecretTypeAccessToken, Value: "ghp_realtoken",
	})
	if err != nil {
		t.Fatalf("StoreSecret: %v", err)
	}
	val, err := resolver.Resolve(ctx, ref, SecretPurposeGitHubAPI)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if val != "ghp_realtoken" {
		t.Errorf("expected resolved plaintext value, got %q", val)
	}

	privateKeyRef, err := svc.StoreSecret(ctx, StoreSecretInput{
		Name: "Deploy Key", Type: SecretTypePrivateKey, Value: "-----BEGIN PRIVATE KEY-----",
	})
	if err != nil {
		t.Fatalf("StoreSecret: %v", err)
	}
	if _, err := resolver.Resolve(ctx, privateKeyRef, SecretPurposeGitHubAPI); err != ErrPurposeNotAllowed {
		t.Errorf("expected ErrPurposeNotAllowed for a private_key resolved as github_api, got %v", err)
	}
}

func TestResolverUnknownSecret(t *testing.T) {
	dir := t.TempDir()
	resolver := NewResolver(dir, providers.NewMemoryProvider())
	_, err := resolver.Resolve(context.Background(), SecretReference{SecretID: "does-not-exist"}, SecretPurposeGeneric)
	if err != ErrSecretNotFound {
		t.Errorf("expected ErrSecretNotFound, got %v", err)
	}
}

func TestPurposeAllowedTable(t *testing.T) {
	if !PurposeAllowed(SecretTypeAccessToken, SecretPurposeGitHubAPI) {
		t.Error("expected access_token to be allowed for github_api")
	}
	if PurposeAllowed(SecretTypePrivateKey, SecretPurposeGitHubAPI) {
		t.Error("expected private_key to NOT be allowed for github_api")
	}
	if PurposeAllowed(SecretTypeAPIKey, "") {
		t.Error("expected an empty/unknown purpose to never be allowed")
	}
}
