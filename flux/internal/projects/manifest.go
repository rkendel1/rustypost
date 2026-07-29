package projects

import (
	"os"
	"path/filepath"

	"flux/internal/storage"
)

// ManifestSchemaVersion is bumped whenever the on-disk shape of Manifest
// changes in a way that requires a migration step to read older files.
const ManifestSchemaVersion = 1

const (
	reqitDirName     = ".reqit"
	manifestFileName = "project.json"
	stateDirName     = "state"
	cacheDirName     = "cache"
	logsDirName      = "logs"
	gitignoreName    = ".gitignore"
)

// Manifest is the portable, non-secret project descriptor stored at
// .reqit/project.json. It must never contain API keys, tokens, passwords,
// private keys, connection strings, or any other secret value — only stable
// project configuration and references (e.g. vault.SecretReference IDs, once
// integrations are wired in Phase 4).
type Manifest struct {
	Version      int             `json:"version"`
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Description  string          `json:"description,omitempty"`
	Status       ProjectStatus   `json:"status,omitempty"`
	Sources      []ProjectSource `json:"sources"`
	Capabilities []string        `json:"capabilities,omitempty"`
	CreatedAt    string          `json:"createdAt"`
	UpdatedAt    string          `json:"updatedAt"`
}

// reqitDir returns the .reqit directory path for a project rooted at dir.
func reqitDir(rootDir string) string {
	return filepath.Join(rootDir, reqitDirName)
}

// ManifestPath returns the path of the portable project manifest.
func ManifestPath(rootDir string) string {
	return filepath.Join(reqitDir(rootDir), manifestFileName)
}

// StateDir, CacheDir, and LogsDir are Reqit-owned local state, separate from
// the portable manifest. They must be gitignored by default.
func StateDir(rootDir string) string { return filepath.Join(reqitDir(rootDir), stateDirName) }
func CacheDir(rootDir string) string { return filepath.Join(reqitDir(rootDir), cacheDirName) }
func LogsDir(rootDir string) string  { return filepath.Join(reqitDir(rootDir), logsDirName) }

// ReadManifest loads the manifest for a project rooted at rootDir. It
// returns a zero-value Manifest (no error) if no manifest file exists yet.
func ReadManifest(rootDir string) (Manifest, error) {
	var m Manifest
	if err := storage.LoadFrom(reqitDir(rootDir), manifestFileName, &m); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

// WriteManifest atomically writes the manifest for a project rooted at
// rootDir.
func WriteManifest(rootDir string, m Manifest) error {
	return storage.SaveTo(reqitDir(rootDir), manifestFileName, m)
}

// EnsureLayout creates the .reqit/{state,cache,logs} directories and a
// .gitignore inside .reqit that excludes them by default, leaving
// project.json (and any collections/specifications/tests/reports the user
// chooses to commit) untouched.
func EnsureLayout(rootDir string) error {
	for _, dir := range []string{StateDir(rootDir), CacheDir(rootDir), LogsDir(rootDir)} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return writeGitignore(rootDir)
}

func writeGitignore(rootDir string) error {
	path := filepath.Join(reqitDir(rootDir), gitignoreName)
	if _, err := os.Stat(path); err == nil {
		return nil // don't clobber a user-edited .gitignore
	}
	// automation/ holds job/pipeline activity history (internal/automation/history)
	// — local runtime state, like state/cache/logs, never portable manifest data.
	contents := stateDirName + "/\n" + cacheDirName + "/\n" + logsDirName + "/\n" + "automation/\n"
	return os.WriteFile(path, []byte(contents), 0o644)
}
