package capabilities

import (
	"context"
	"testing"

	"flux/internal/integrations"
	"flux/internal/projects"
	"flux/internal/vault"
	vaultproviders "flux/internal/vault/providers"
)

func TestRegistrySnapshot(t *testing.T) {
	reg := NewRegistry()
	reg.Register(NewAPIIntelligenceCapability())

	ctx := context.Background()
	root := t.TempDir()
	svc := projects.NewService(t.TempDir())
	p, err := svc.Create(ctx, projects.CreateProjectInput{Name: "Test", RootDir: root})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	snap := reg.Snapshot(ctx, *p)
	if len(snap) != 1 {
		t.Fatalf("expected 1 snapshot entry, got %d", len(snap))
	}
	if snap[0].ID != "api_intelligence" {
		t.Errorf("unexpected capability ID: %s", snap[0].ID)
	}
}

func TestAPIIntelligenceAvailability(t *testing.T) {
	cap := NewAPIIntelligenceCapability()
	ctx := context.Background()
	root := t.TempDir()
	svc := projects.NewService(t.TempDir())

	p, err := svc.Create(ctx, projects.CreateProjectInput{Name: "Test", RootDir: root})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got := cap.Availability(ctx, *p); got != CapabilityRequiresSetup {
		t.Errorf("expected requires_setup with no sources, got %s", got)
	}

	if _, err := svc.AddSource(ctx, p.ID, projects.AddSourceInput{Name: "root", Kind: projects.SourceTypeLocalFolder, Path: "."}); err != nil {
		t.Fatalf("AddSource: %v", err)
	}
	opened, err := svc.Open(ctx, p.ID)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if got := cap.Availability(ctx, *opened); got != CapabilityAvailable {
		t.Errorf("expected available with a resolvable source, got %s", got)
	}
}

func TestGitHubCapabilityAvailability(t *testing.T) {
	ctx := context.Background()
	vaultDir := t.TempDir()
	mem := vaultproviders.NewMemoryProvider()
	vaultSvc := vault.NewService(vaultDir, mem)
	resolver := vault.NewResolver(vaultDir, mem)
	integrationSvc := integrations.NewService(t.TempDir(), resolver)

	cap := NewGitHubCapability(integrationSvc)

	projSvc := projects.NewService(t.TempDir())
	p, err := projSvc.Create(ctx, projects.CreateProjectInput{Name: "Test", RootDir: t.TempDir()})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if got := cap.Availability(ctx, *p); got != CapabilityRequiresSetup {
		t.Errorf("expected requires_setup with no GitHub integration, got %s", got)
	}

	ref, err := vaultSvc.StoreSecret(ctx, vault.StoreSecretInput{Name: "gh", Type: vault.SecretTypeAccessToken, Value: "ghp_x"})
	if err != nil {
		t.Fatalf("StoreSecret: %v", err)
	}
	if _, err := integrationSvc.Create(ctx, integrations.CreateIntegrationInput{
		Provider: integrations.ProviderGitHub, Name: "GitHub (default)", Credential: ref,
	}); err != nil {
		t.Fatalf("Create integration: %v", err)
	}

	if got := cap.Availability(ctx, *p); got != CapabilityAvailable {
		t.Errorf("expected available once a connected GitHub integration exists, got %s", got)
	}
}

func TestGitHubCapabilityUnavailableWithNilService(t *testing.T) {
	cap := NewGitHubCapability(nil)
	if got := cap.Availability(context.Background(), projects.Project{}); got != CapabilityUnavailable {
		t.Errorf("expected unavailable with a nil integration service, got %s", got)
	}
}
