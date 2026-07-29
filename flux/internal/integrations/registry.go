package integrations

import (
	"sync"

	"flux/internal/storage"
)

const registryFile = "integrations.json"

type registryIndex struct {
	Integrations []Integration `json:"integrations"`
}

// registry persists Integration records — never secret values, which live
// only as vault.SecretReferences — to a JSON file scoped to an explicit
// base directory. Like vault's registry, it re-reads on every operation
// rather than caching, since IntegrationService may coexist with other
// long-lived readers.
type registry struct {
	mu      sync.Mutex
	baseDir string
}

func newRegistry(baseDir string) *registry { return &registry{baseDir: baseDir} }

func (r *registry) read() ([]Integration, error) {
	var idx registryIndex
	if err := storage.LoadFrom(r.baseDir, registryFile, &idx); err != nil {
		return nil, err
	}
	if idx.Integrations == nil {
		idx.Integrations = []Integration{}
	}
	return idx.Integrations, nil
}

func (r *registry) write(entries []Integration) error {
	return storage.SaveTo(r.baseDir, registryFile, registryIndex{Integrations: entries})
}

func (r *registry) list(projectID string) ([]Integration, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entries, err := r.read()
	if err != nil {
		return nil, err
	}
	if projectID == "" {
		return entries, nil
	}
	out := make([]Integration, 0, len(entries))
	for _, e := range entries {
		if e.ProjectID == projectID {
			out = append(out, e)
		}
	}
	return out, nil
}

func (r *registry) get(id string) (Integration, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entries, err := r.read()
	if err != nil {
		return Integration{}, err
	}
	for _, e := range entries {
		if e.ID == id {
			return e, nil
		}
	}
	return Integration{}, ErrIntegrationNotFound
}

func (r *registry) put(in Integration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	entries, err := r.read()
	if err != nil {
		return err
	}
	for i, e := range entries {
		if e.ID == in.ID {
			entries[i] = in
			return r.write(entries)
		}
	}
	entries = append(entries, in)
	return r.write(entries)
}

func (r *registry) remove(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	entries, err := r.read()
	if err != nil {
		return err
	}
	for i, e := range entries {
		if e.ID == id {
			entries = append(entries[:i], entries[i+1:]...)
			return r.write(entries)
		}
	}
	return ErrIntegrationNotFound
}
