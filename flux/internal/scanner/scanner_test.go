package scanner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanRepositoryDiscoversEndpointsAndAuth(t *testing.T) {
	root := t.TempDir()
	src := `package api
func register(router any) {
  // Authorization: ******
  router.Get("/users/{id}", nil)
  router.Post("/users", nil)
}`
	if err := os.WriteFile(filepath.Join(root, "routes.go"), []byte(src), 0o644); err != nil {
		t.Fatalf("write routes.go: %v", err)
	}

	inv, err := ScanRepository(root)
	if err != nil {
		t.Fatalf("ScanRepository: %v", err)
	}
	if len(inv.Endpoints) != 2 {
		t.Fatalf("expected 2 endpoints, got %d", len(inv.Endpoints))
	}
	if len(inv.AuthSchemes) == 0 || inv.AuthSchemes[0] != "bearer" {
		t.Fatalf("expected bearer auth scheme, got %+v", inv.AuthSchemes)
	}
}

func TestGenerateArtifactsCreatesScanOutputs(t *testing.T) {
	root := t.TempDir()
	inv := &Inventory{
		GeneratedAt:  "2026-01-01T00:00:00Z",
		Repository:   root,
		FilesScanned: 1,
		Endpoints: []Endpoint{
			{
				Method:      "GET",
				Path:        "/health",
				SourceFiles: []string{"routes.go"},
				LineNumbers: []int{1},
				ResponseSchemas: map[string]map[string]any{
					"200": {"type": "object", "additionalProperties": true},
				},
			},
		},
	}

	out := filepath.Join(root, ".reqit", "scan")
	artifacts, err := GenerateArtifacts(root, out, inv)
	if err != nil {
		t.Fatalf("GenerateArtifacts: %v", err)
	}

	if _, err := os.Stat(artifacts.OpenAPIPath); err != nil {
		t.Fatalf("missing OpenAPI file: %v", err)
	}
	b, err := os.ReadFile(artifacts.OpenAPIPath)
	if err != nil {
		t.Fatalf("read OpenAPI file: %v", err)
	}
	if !strings.Contains(string(b), `"openapi": "3.1.0"`) {
		t.Fatalf("openapi version missing from generated file")
	}
	if _, err := os.Stat(artifacts.HarnessPath); err != nil {
		t.Fatalf("missing harness file: %v", err)
	}
}
