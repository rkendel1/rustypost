package projects

import (
	"errors"
	"sync"
	"time"

	"flux/internal/storage"
)

const registryIndexFile = "projects.json"

// ErrProjectNotFound is returned when a project ID has no matching registry entry.
var ErrProjectNotFound = errors.New("project not found")

// registryEntry is a row in the local machine-wide project index: it records
// where a project's root directory lives so the app can find it again
// without scanning the filesystem. The portable project data itself lives in
// the manifest at <RootDir>/.reqit/project.json.
type registryEntry struct {
	ID           string `json:"id"`
	RootDir      string `json:"rootDir"`
	LastOpenedAt string `json:"lastOpenedAt"`
}

type registryIndex struct {
	Active   string          `json:"active"`
	Projects []registryEntry `json:"projects"`
}

// registry is the on-disk index of known projects, analogous to
// workspaces.Store but scoped to Projects. Unlike workspaces.Store, it takes
// its base directory explicitly rather than resolving a process-wide
// singleton AppDir, so callers (and tests) control exactly where the index
// lives.
type registry struct {
	mu      sync.Mutex
	baseDir string
	entries []registryEntry
	active  string
	loaded  bool
}

func newRegistry(baseDir string) *registry { return &registry{baseDir: baseDir} }

func (r *registry) load() error {
	if r.loaded {
		return nil
	}
	var idx registryIndex
	if err := storage.LoadFrom(r.baseDir, registryIndexFile, &idx); err != nil {
		return err
	}
	if idx.Projects == nil {
		idx.Projects = []registryEntry{}
	}
	r.entries = idx.Projects
	r.active = idx.Active
	r.loaded = true
	return nil
}

func (r *registry) save() error {
	return storage.SaveTo(r.baseDir, registryIndexFile, registryIndex{Active: r.active, Projects: r.entries})
}

func (r *registry) find(id string) (registryEntry, bool) {
	for _, e := range r.entries {
		if e.ID == id {
			return e, true
		}
	}
	return registryEntry{}, false
}

// list returns all registry entries, loading the index first.
func (r *registry) list() ([]registryEntry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.load(); err != nil {
		return nil, err
	}
	out := make([]registryEntry, len(r.entries))
	copy(out, r.entries)
	return out, nil
}

// upsert inserts a new entry or updates RootDir for an existing one, keyed
// by ID. Used by both Create and migration (which reuses a stable, known ID
// so re-running migration is a no-op rather than a duplicate).
func (r *registry) upsert(id, rootDir string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.load(); err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	for i, e := range r.entries {
		if e.ID == id {
			r.entries[i].RootDir = rootDir
			return r.save()
		}
	}
	r.entries = append(r.entries, registryEntry{ID: id, RootDir: rootDir, LastOpenedAt: now})
	if r.active == "" {
		r.active = id
	}
	return r.save()
}

func (r *registry) touch(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.load(); err != nil {
		return err
	}
	for i, e := range r.entries {
		if e.ID == id {
			r.entries[i].LastOpenedAt = time.Now().UTC().Format(time.RFC3339)
			r.active = id
			return r.save()
		}
	}
	return ErrProjectNotFound
}

func (r *registry) remove(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.load(); err != nil {
		return err
	}
	for i, e := range r.entries {
		if e.ID == id {
			r.entries = append(r.entries[:i], r.entries[i+1:]...)
			if r.active == id {
				r.active = ""
			}
			return r.save()
		}
	}
	return ErrProjectNotFound
}

// activeID returns the currently active project's ID, or "" if none is set
// (e.g. a brand new install with nothing migrated or created yet).
func (r *registry) activeID() (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.load(); err != nil {
		return "", err
	}
	return r.active, nil
}

func (r *registry) get(id string) (registryEntry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.load(); err != nil {
		return registryEntry{}, err
	}
	e, ok := r.find(id)
	if !ok {
		return registryEntry{}, ErrProjectNotFound
	}
	return e, nil
}
