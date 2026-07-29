package workspace

import (
	"path/filepath"
	"testing"
)

func TestManagerMetadataRoundTrip(t *testing.T) {
	root := t.TempDir()
	m := NewManager(root)

	info := Info{
		Name:       "Customer API",
		Repository: "github.com/acme/customer-api",
		Branch:     "main",
		Framework:  "FastAPI",
		Endpoints:  132,
		Status:     "Healthy",
	}
	if err := m.SaveInfo(info); err != nil {
		t.Fatalf("save info: %v", err)
	}
	got, err := m.LoadInfo()
	if err != nil {
		t.Fatalf("load info: %v", err)
	}
	if got.Repository != info.Repository || got.Framework != info.Framework || got.Endpoints != info.Endpoints {
		t.Fatalf("unexpected info: %#v", got)
	}

	if err := m.AppendScanHistory(ScanHistory{FilesScanned: 20, Endpoints: 5}, 3); err != nil {
		t.Fatalf("append history: %v", err)
	}
	if err := m.AppendScanHistory(ScanHistory{FilesScanned: 21, Endpoints: 6}, 3); err != nil {
		t.Fatalf("append history: %v", err)
	}
	history, err := m.LoadScanHistory()
	if err != nil {
		t.Fatalf("load history: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 history rows, got %d", len(history))
	}

	if err := m.SaveSettings(map[string]any{"ci": map[string]any{"enabled": true}}); err != nil {
		t.Fatalf("save settings: %v", err)
	}
	settings, err := m.LoadSettings()
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}
	if _, ok := settings["ci"]; !ok {
		t.Fatalf("expected ci settings")
	}

	cache := map[string]any{"routes": 10}
	if err := m.WriteCache("routes.json", cache); err != nil {
		t.Fatalf("write cache: %v", err)
	}
	var cacheOut map[string]any
	if err := m.ReadCache("routes.json", &cacheOut); err != nil {
		t.Fatalf("read cache: %v", err)
	}
	routes, ok := cacheOut["routes"].(float64)
	if !ok || routes != 10 {
		t.Fatalf("unexpected cache data: %#v", cacheOut)
	}
}

func TestEnsureTargetArtifactLayout(t *testing.T) {
	root := t.TempDir()
	layout, err := EnsureTargetArtifactLayout(root)
	if err != nil {
		t.Fatalf("ensure layout: %v", err)
	}
	dirs := []string{
		layout.CollectionsDir,
		layout.EnvironmentsDir,
		layout.ReportsDir,
		layout.OpenAPIDir,
		layout.TestsDir,
		layout.WorkflowsDir,
	}
	for _, dir := range dirs {
		if !filepath.IsAbs(dir) {
			t.Fatalf("expected absolute path for %s", dir)
		}
	}
}
