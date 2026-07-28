package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunScanCommand(t *testing.T) {
	repo := t.TempDir()
	content := `package main
func setup(r any) { r.Get("/status", nil) }`
	if err := os.WriteFile(filepath.Join(repo, "routes.go"), []byte(content), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}
	out := filepath.Join(repo, "scan-output")

	code := Run([]string{"scan", repo, "--output", out})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	expected := []string{
		filepath.Join(out, "openapi.json"),
		filepath.Join(out, "workspace.json"),
		filepath.Join(out, "inventory.json"),
		filepath.Join(out, "testsuites.json"),
		filepath.Join(out, "drift.json"),
		filepath.Join(out, "tests", "scan-harness.js"),
		filepath.Join(out, "tests", "scan.playwright.spec.js"),
		filepath.Join(out, "tests", "scan.jest.spec.js"),
		filepath.Join(out, "tests", "scan.runner.js"),
	}
	for _, p := range expected {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("expected artifact %s: %v", p, err)
		}
	}
}
