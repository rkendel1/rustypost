package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTestRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	content := `package main
func setup(r any) {
  // Authorization: ******
  r.Get("/status", nil)
  r.Post("/payments", nil)
}`
	if err := os.WriteFile(filepath.Join(repo, "routes.go"), []byte(content), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}
	return repo
}

func TestRunGenerateCommand(t *testing.T) {
	repo := writeTestRepo(t)
	out := filepath.Join(repo, "scan-output")

	code := Run([]string{"generate", repo, "--output", out})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if _, err := os.Stat(filepath.Join(out, "openapi.json")); err != nil {
		t.Fatalf("expected openapi.json: %v", err)
	}
}

func TestRunHealthCommandWritesHealthReport(t *testing.T) {
	repo := writeTestRepo(t)
	out := filepath.Join(repo, "scan-output")

	code := Run([]string{"health", repo, "--output", out})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if _, err := os.Stat(filepath.Join(out, "health.json")); err != nil {
		t.Fatalf("expected health.json: %v", err)
	}
}

func TestRunWorkflowInstallCommand(t *testing.T) {
	repo := writeTestRepo(t)
	out := filepath.Join(repo, "scan-output")

	code := Run([]string{"workflow", "install", repo, "--output", out})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	expected := []string{
		filepath.Join(repo, ".github", "workflows", "api-smoke.yml"),
		filepath.Join(repo, ".github", "workflows", "api-contract.yml"),
		filepath.Join(repo, ".github", "workflows", "api-regression.yml"),
		filepath.Join(repo, ".github", "workflows", "api-security.yml"),
		filepath.Join(repo, ".github", "workflows", "schema-drift.yml"),
		filepath.Join(repo, ".github", "workflows", "sdk-generation.yml"),
	}
	for _, p := range expected {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("expected workflow file %s: %v", p, err)
		}
	}
}

func TestRunSDKGenerateCommand(t *testing.T) {
	repo := writeTestRepo(t)
	out := filepath.Join(repo, "scan-output")

	code := Run([]string{"sdk", "generate", repo, "--output", out})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	expected := []string{
		filepath.Join(out, "sdk", "manifest.json"),
		filepath.Join(out, "sdk", "typescript.ts"),
		filepath.Join(out, "sdk", "python.py"),
		filepath.Join(out, "sdk", "go.go"),
		filepath.Join(out, "sdk", "rust.rs"),
	}
	for _, p := range expected {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("expected sdk file %s: %v", p, err)
		}
	}
}

func TestRunReportCommandWritesFormats(t *testing.T) {
	repo := writeTestRepo(t)
	out := filepath.Join(repo, "scan-output")

	code := Run([]string{"report", repo, "--output", out})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	expected := []string{
		filepath.Join(out, "report.json"),
		filepath.Join(out, "report.md"),
		filepath.Join(out, "report.html"),
	}
	for _, p := range expected {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("expected report file %s: %v", p, err)
		}
	}
}
