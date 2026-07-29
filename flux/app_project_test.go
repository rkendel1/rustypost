package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"flux/internal/projects"
)

// TestMain redirects storage.AppDir() (used internally by workspaces.Store,
// which these tests exercise via a.workspaces) into a throwaway sandbox for
// the whole test binary, so these tests never read or write the real user
// config directory. storage.AppDir() caches its result via sync.Once, so
// this must happen before any test calls it.
func TestMain(m *testing.M) {
	sandbox, err := os.MkdirTemp("", "reqit-app-test-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(sandbox)
	os.Setenv("HOME", sandbox)
	os.Setenv("XDG_CONFIG_HOME", filepath.Join(sandbox, "config"))
	os.Exit(m.Run())
}

// TestMountProjectMountsActiveSourceDir verifies the Phase 3 vertical slice:
// mountProject resolves a Project's active source directory and mounts it
// through the exact same store-attachment logic mountWorkspace already
// provided, so opening a Project produces a fully-functional app session
// exactly like opening a workspace did.
func TestMountProjectMountsActiveSourceDir(t *testing.T) {
	root := t.TempDir()
	a := NewApp()
	a.ctx = context.Background()
	a.projects = projects.NewService(t.TempDir())

	t.Cleanup(func() {
		if a.fsWatcher != nil {
			a.fsWatcher.Close()
		}
		if a.schedulerExec != nil {
			a.schedulerExec.Stop()
		}
	})

	p, err := a.projects.Create(a.ctx, projects.CreateProjectInput{Name: "FieldFlow", RootDir: root})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := a.projects.AddSource(a.ctx, p.ID, projects.AddSourceInput{
		Name: "fieldflow-api", Kind: projects.SourceTypeLocalFolder, Path: ".",
	}); err != nil {
		t.Fatalf("AddSource: %v", err)
	}
	opened, err := a.projects.Open(a.ctx, p.ID)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	a.mountProject(opened)

	if a.activeProject == nil || a.activeProject.ID != p.ID {
		t.Fatalf("expected activeProject to be set to %s, got %+v", p.ID, a.activeProject)
	}
	if a.collections == nil {
		t.Fatal("expected mountProject to mount the collections store via mountWorkspace")
	}
	if a.history == nil || a.environments == nil {
		t.Fatal("expected mountProject to mount history and environments stores via mountWorkspace")
	}
}

// TestMigrateFromWorkspacesThenMountAtStartup exercises the same sequence
// app.go's startup() performs: migrate legacy workspaces into Projects, find
// the active one, and mount it — proving a user with an existing workspace
// gets an equivalent, fully-mounted session after this PR with no manual
// action required.
func TestMigrateFromWorkspacesThenMountAtStartup(t *testing.T) {
	a := NewApp()
	a.ctx = context.Background()
	a.projects = projects.NewService(t.TempDir())

	t.Cleanup(func() {
		if a.fsWatcher != nil {
			a.fsWatcher.Close()
		}
		if a.schedulerExec != nil {
			a.schedulerExec.Stop()
		}
	})

	info, err := a.workspaces.Create("Legacy", "desc", "#3B82F6")
	if err != nil {
		t.Fatalf("workspaces.Create: %v", err)
	}

	if err := projects.MigrateFromWorkspaces(a.projects, a.workspaces); err != nil {
		t.Fatalf("MigrateFromWorkspaces: %v", err)
	}
	id, ok := projects.ActiveProjectID(a.projects)
	if !ok {
		t.Fatal("expected an active project after migrating a workspace")
	}
	if id != info.ID {
		t.Errorf("expected active project to reuse the workspace ID %s, got %s", info.ID, id)
	}
	p, err := a.projects.Open(a.ctx, id)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	a.mountProject(p)

	if a.activeProject == nil || a.activeProject.ID != info.ID {
		t.Fatal("expected the migrated project to be mounted as active")
	}
	if a.collections == nil {
		t.Fatal("expected the migrated project's collections store to be mounted")
	}
}
