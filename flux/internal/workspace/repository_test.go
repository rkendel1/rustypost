package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDeriveRepoDirName(t *testing.T) {
	cases := map[string]string{
		"https://github.com/acme/customer-api.git": "customer-api",
		"git@github.com:acme/customer-api.git":     "customer-api",
		"customer-api":                             "customer-api",
	}
	for in, want := range cases {
		if got := deriveRepoDirName(in); got != want {
			t.Fatalf("deriveRepoDirName(%q)=%q want %q", in, got, want)
		}
	}
}

func TestRepositoryServiceOpenScanAndChanges(t *testing.T) {
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "routes.go"), []byte(`package main; func setup(r any){ r.Get("/status", nil) }`), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	svc := NewRepositoryService(NewManager(t.TempDir()))
	if err := svc.Open(repo); err != nil {
		t.Fatalf("open repo: %v", err)
	}
	inv, err := svc.Scan()
	if err != nil {
		t.Fatalf("scan repo: %v", err)
	}
	if inv.FilesScanned == 0 {
		t.Fatalf("expected scanned files")
	}
	changed, err := svc.DetectChanges()
	if err != nil {
		t.Fatalf("detect changes: %v", err)
	}
	if !changed {
		t.Fatalf("expected uncommitted changes in initialized repo")
	}
}
