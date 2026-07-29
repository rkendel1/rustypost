package projects

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"flux/internal/workspaces"
)

// TestMain redirects storage.AppDir() into a throwaway sandbox for the whole
// test binary. Our own registry takes its base directory explicitly (see
// newTestService), but internal/workspaces.Store still resolves the
// process-wide AppDir singleton internally, and the migration tests below
// construct a real workspaces.Store — this ensures they never read or write
// the real user config directory. storage.AppDir() caches its result via
// sync.Once, so this must happen before any test calls it.
func TestMain(m *testing.M) {
	sandbox, err := os.MkdirTemp("", "reqit-projects-test-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(sandbox)
	os.Setenv("HOME", sandbox)
	os.Setenv("XDG_CONFIG_HOME", filepath.Join(sandbox, "config"))
	os.Exit(m.Run())
}

// newTestService returns a ProjectService backed by a registry index scoped
// to its own temp directory, so tests never see each other's projects.
func newTestService(t *testing.T) ProjectService {
	t.Helper()
	return NewService(t.TempDir())
}

func TestCreateOpenList(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()
	root := t.TempDir()

	p, err := svc.Create(ctx, CreateProjectInput{Name: "FieldFlow", Description: "Field ops", RootDir: root})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if p.ID == "" {
		t.Fatal("expected a generated project ID")
	}
	if p.Status != ProjectStatusActive {
		t.Errorf("expected active status, got %s", p.Status)
	}

	opened, err := svc.Open(ctx, p.ID)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if opened.Name != "FieldFlow" {
		t.Errorf("expected name FieldFlow, got %s", opened.Name)
	}

	list, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0].ID != p.ID {
		t.Fatalf("expected exactly the created project in List, got %+v", list)
	}
}

func TestCreateRequiresNameAndRootDir(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	if _, err := svc.Create(ctx, CreateProjectInput{RootDir: t.TempDir()}); err != ErrNameRequired {
		t.Errorf("expected ErrNameRequired, got %v", err)
	}
	if _, err := svc.Create(ctx, CreateProjectInput{Name: "X"}); err != ErrRootDirRequired {
		t.Errorf("expected ErrRootDirRequired, got %v", err)
	}
}

func TestUpdateAndArchive(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()
	root := t.TempDir()

	p, err := svc.Create(ctx, CreateProjectInput{Name: "Orig", RootDir: root})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	newName := "Renamed"
	updated, err := svc.Update(ctx, p.ID, UpdateProjectInput{Name: &newName})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Name != "Renamed" {
		t.Errorf("expected renamed project, got %s", updated.Name)
	}

	if err := svc.Archive(ctx, p.ID); err != nil {
		t.Fatalf("Archive: %v", err)
	}
	afterArchive, err := svc.Open(ctx, p.ID)
	if err != nil {
		t.Fatalf("Open after archive: %v", err)
	}
	if afterArchive.Status != ProjectStatusArchived {
		t.Errorf("expected archived status to persist, got %s", afterArchive.Status)
	}
}

func TestAddAndRemoveSource(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()
	root := t.TempDir()

	p, err := svc.Create(ctx, CreateProjectInput{Name: "Multi", RootDir: root})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	src, err := svc.AddSource(ctx, p.ID, AddSourceInput{Name: "api", Kind: SourceTypeLocalFolder, Path: "."})
	if err != nil {
		t.Fatalf("AddSource: %v", err)
	}
	if src.ID == "" {
		t.Fatal("expected a generated source ID")
	}

	opened, err := svc.Open(ctx, p.ID)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if len(opened.Sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(opened.Sources))
	}

	if err := svc.RemoveSource(ctx, p.ID, src.ID); err != nil {
		t.Fatalf("RemoveSource: %v", err)
	}
	opened, err = svc.Open(ctx, p.ID)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if len(opened.Sources) != 0 {
		t.Fatalf("expected 0 sources after removal, got %d", len(opened.Sources))
	}

	if err := svc.RemoveSource(ctx, p.ID, "does-not-exist"); err != ErrSourceNotFound {
		t.Errorf("expected ErrSourceNotFound, got %v", err)
	}
}

func TestAddSourceRejectsPathEscape(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()
	root := t.TempDir()

	p, err := svc.Create(ctx, CreateProjectInput{Name: "Escape", RootDir: root})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := svc.AddSource(ctx, p.ID, AddSourceInput{Name: "escapee", Kind: SourceTypeLocalFolder, Path: "../../etc"}); err == nil {
		t.Error("expected path traversal to be rejected")
	}
}

func TestManifestRoundTrip(t *testing.T) {
	root := t.TempDir()
	m := Manifest{
		Version:   ManifestSchemaVersion,
		ID:        "proj_1",
		Name:      "Test",
		Status:    ProjectStatusActive,
		Sources:   []ProjectSource{{ID: "src_1", Name: "api", Kind: SourceTypeGitRepository, URL: "https://example.com/repo.git"}},
		CreatedAt: "2026-01-01T00:00:00Z",
		UpdatedAt: "2026-01-01T00:00:00Z",
	}
	if err := WriteManifest(root, m); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}
	loaded, err := ReadManifest(root)
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
	if loaded.ID != m.ID || loaded.Name != m.Name || len(loaded.Sources) != 1 {
		t.Errorf("round trip mismatch: %+v", loaded)
	}
	if loaded.Sources[0].URL != "https://example.com/repo.git" {
		t.Errorf("expected source URL to survive round trip, got %q", loaded.Sources[0].URL)
	}
}

func TestManifestNeverContainsSecretLikeFields(t *testing.T) {
	root := t.TempDir()
	svc := newTestService(t)
	_, err := svc.Create(context.Background(), CreateProjectInput{Name: "PlainProject", RootDir: root})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	data, err := os.ReadFile(ManifestPath(root))
	if err != nil {
		t.Fatalf("reading manifest: %v", err)
	}
	forbidden := []string{"token", "password", "secret", "apiKey", "privateKey"}
	content := string(data)
	for _, f := range forbidden {
		if containsFold(content, f) {
			t.Errorf("manifest unexpectedly contains %q: %s", f, content)
		}
	}
}

func containsFold(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		match := true
		for j := 0; j < len(needle); j++ {
			a, b := haystack[i+j], needle[j]
			if a >= 'A' && a <= 'Z' {
				a += 'a' - 'A'
			}
			if b >= 'A' && b <= 'Z' {
				b += 'a' - 'A'
			}
			if a != b {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func TestEnsureLayoutGitignore(t *testing.T) {
	root := t.TempDir()
	if err := EnsureLayout(root); err != nil {
		t.Fatalf("EnsureLayout: %v", err)
	}
	for _, dir := range []string{StateDir(root), CacheDir(root), LogsDir(root)} {
		if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
			t.Errorf("expected %s to exist as a directory", dir)
		}
	}
	gitignore, err := os.ReadFile(filepath.Join(root, ".reqit", ".gitignore"))
	if err != nil {
		t.Fatalf("reading .gitignore: %v", err)
	}
	for _, want := range []string{"state/", "cache/", "logs/", "automation/"} {
		if !containsFold(string(gitignore), want) {
			t.Errorf("expected .gitignore to exclude %s, got: %s", want, gitignore)
		}
	}
}

// TestMigrateFromWorkspaces exercises both the noop-on-empty-store case and
// the idempotent/non-destructive migration case in a single test function
// (rather than two), because workspaces.Store resolves a process-wide
// storage.AppDir() singleton internally — running these as separate test
// functions would make the "noop on empty" assertion depend on test
// execution order relative to the "migrates and is idempotent" assertion.
func TestMigrateFromWorkspaces(t *testing.T) {
	ws := workspaces.NewStore()
	svc := newTestService(t)

	// 1. Noop on an empty workspace store (must run before any workspace exists).
	if err := MigrateFromWorkspaces(svc, ws); err != nil {
		t.Fatalf("expected no error migrating an empty workspace store: %v", err)
	}
	list, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected no projects from an empty workspace store, got %d", len(list))
	}

	// 2. Create a legacy workspace, then migrate it.
	info, err := ws.Create("Legacy", "a legacy workspace", "#3B82F6")
	if err != nil {
		t.Fatalf("workspaces.Create: %v", err)
	}
	if err := MigrateFromWorkspaces(svc, ws); err != nil {
		t.Fatalf("first migration: %v", err)
	}
	list, err = svc.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected exactly 1 migrated project, got %d", len(list))
	}
	if list[0].ID != info.ID {
		t.Errorf("expected migrated project to reuse workspace ID %s, got %s", info.ID, list[0].ID)
	}

	// 3. Re-running migration must not create a duplicate.
	if err := MigrateFromWorkspaces(svc, ws); err != nil {
		t.Fatalf("second migration: %v", err)
	}
	list, err = svc.List(context.Background())
	if err != nil {
		t.Fatalf("List after re-migration: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected migration to remain idempotent, got %d projects", len(list))
	}

	// 4. The legacy workspace data must still be intact and functional.
	stillThere, err := ws.GetAll()
	if err != nil {
		t.Fatalf("workspaces.GetAll after migration: %v", err)
	}
	if len(stillThere) != 1 || stillThere[0].ID != info.ID {
		t.Error("expected legacy workspace data to remain untouched after migration")
	}
}

func TestRecordAndReadEvents(t *testing.T) {
	root := t.TempDir()
	if err := RecordEvent(root, ProjectEvent{ProjectID: "p1", Type: EventProjectCreated, Summary: "Project created"}); err != nil {
		t.Fatalf("RecordEvent: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(StateDir(root), activityLogFile))
	if err != nil {
		t.Fatalf("reading activity log: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected activity log to contain the recorded event")
	}
}
