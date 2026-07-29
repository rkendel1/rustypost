package projects

import (
	"errors"
	"os"
	"path/filepath"

	"flux/internal/security"
)

// MountedProject resolves a Project into the concrete local paths the rest
// of the app needs to attach its per-project stores (collections, history,
// environments, git, scheduler, ...). It is the project-scoped analog of
// what app.go's mountWorkspace historically did with a raw workspace
// directory.
type MountedProject struct {
	Project *Project

	RootDir  string
	StateDir string
	CacheDir string
	LogsDir  string

	// ActiveSourcePath is the resolved, absolute path of the project's
	// primary source (its first source, today) — the directory dependent
	// stores should be mounted against. Falls back to RootDir if the
	// project has no sources yet.
	ActiveSourcePath string
}

// Mount ensures a project's local layout exists and resolves its active
// source path. It does not attach any stores itself — callers (app.go) do
// that with the returned paths, exactly as mountWorkspace did with a bare
// directory string.
func Mount(p *Project) (*MountedProject, error) {
	if p == nil {
		return nil, errors.New("project is required")
	}
	if err := EnsureLayout(p.RootDir); err != nil {
		return nil, err
	}
	activePath := p.RootDir
	if len(p.Sources) > 0 {
		if resolved, err := ResolveSourcePath(p.RootDir, p.Sources[0]); err == nil {
			activePath = resolved
		}
	}
	return &MountedProject{
		Project:          p,
		RootDir:          p.RootDir,
		StateDir:         StateDir(p.RootDir),
		CacheDir:         CacheDir(p.RootDir),
		LogsDir:          LogsDir(p.RootDir),
		ActiveSourcePath: activePath,
	}, nil
}

// ResolveSourcePath resolves a ProjectSource to an absolute local path.
// Only local_folder and generated_app sources resolve to a filesystem path;
// git_repository/imported sources without a local checkout return an error
// (the caller should fall back to RootDir or skip filesystem-dependent work).
func ResolveSourcePath(rootDir string, src ProjectSource) (string, error) {
	switch src.Kind {
	case SourceTypeLocalFolder, SourceTypeGeneratedApp:
		if src.Path == "" || src.Path == "." {
			return rootDir, nil
		}
		abs := filepath.Join(rootDir, src.Path)
		if err := security.ValidatePathWithinDir(rootDir, abs); err != nil {
			return "", err
		}
		return abs, nil
	case SourceTypeGitRepository:
		if src.Path != "" {
			abs := filepath.Join(rootDir, src.Path)
			if err := security.ValidatePathWithinDir(rootDir, abs); err != nil {
				return "", err
			}
			return abs, nil
		}
		return "", errors.New("git repository source has no local checkout path")
	default:
		return "", errors.New("source has no resolvable local path")
	}
}

// DetectSourceKind inspects a local filesystem path and returns the most
// likely SourceType for it: git_repository if it has a .git directory,
// otherwise local_folder. It's a heuristic used whenever a Project/Source
// needs to be created from a bare path — legacy workspace migration, or
// wrapping a manually chosen folder ahead of a full Source-picker UI.
func DetectSourceKind(path string) SourceType {
	if _, err := os.Stat(filepath.Join(path, ".git")); err == nil {
		return SourceTypeGitRepository
	}
	return SourceTypeLocalFolder
}
