package vault

import (
	"sync"
	"time"

	"flux/internal/storage"
)

const registryFile = "vault_secrets.json"

// registry persists SecretMetadata only — never values — to a JSON file
// scoped to an explicit base directory (matching internal/projects's
// registry: injected, not a process-wide singleton, so it stays testable).
//
// Unlike internal/projects's registry, this one re-reads the file on every
// operation rather than caching it in memory after first load. VaultService
// and SecretResolver are deliberately separate long-lived instances (that
// separation is the security boundary preventing bindings_*.go from ever
// reaching a resolver) — each pointed at the same underlying file. A
// load-once cache would mean a resolver constructed before a given secret
// was stored (or after it was rotated) via VaultService would never see the
// change for the rest of the process's lifetime. The file is small and
// operations are infrequent, so the extra disk read is the right trade.
type registry struct {
	mu      sync.Mutex
	baseDir string
}

func newRegistry(baseDir string) *registry { return &registry{baseDir: baseDir} }

type registryIndex struct {
	Secrets []SecretMetadata `json:"secrets"`
}

func (r *registry) read() ([]SecretMetadata, error) {
	var idx registryIndex
	if err := storage.LoadFrom(r.baseDir, registryFile, &idx); err != nil {
		return nil, err
	}
	if idx.Secrets == nil {
		idx.Secrets = []SecretMetadata{}
	}
	return idx.Secrets, nil
}

func (r *registry) write(entries []SecretMetadata) error {
	return storage.SaveTo(r.baseDir, registryFile, registryIndex{Secrets: entries})
}

func (r *registry) list(projectID string) ([]SecretMetadata, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entries, err := r.read()
	if err != nil {
		return nil, err
	}
	if projectID == "" {
		return entries, nil
	}
	out := make([]SecretMetadata, 0, len(entries))
	for _, e := range entries {
		if e.ProjectID == projectID {
			out = append(out, e)
		}
	}
	return out, nil
}

func (r *registry) get(id string) (SecretMetadata, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entries, err := r.read()
	if err != nil {
		return SecretMetadata{}, err
	}
	for _, e := range entries {
		if e.ID == id {
			return e, nil
		}
	}
	return SecretMetadata{}, ErrSecretNotFound
}

func (r *registry) put(m SecretMetadata) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	entries, err := r.read()
	if err != nil {
		return err
	}
	for i, e := range entries {
		if e.ID == m.ID {
			entries[i] = m
			return r.write(entries)
		}
	}
	entries = append(entries, m)
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
	return ErrSecretNotFound
}

func (r *registry) touchUsed(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	entries, err := r.read()
	if err != nil {
		return err
	}
	for i, e := range entries {
		if e.ID == id {
			entries[i].LastUsedAt = time.Now().UTC().Format(time.RFC3339)
			return r.write(entries)
		}
	}
	return ErrSecretNotFound
}
