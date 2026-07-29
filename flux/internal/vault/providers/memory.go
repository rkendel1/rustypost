package providers

import "sync"

// MemoryProvider is an in-memory CredentialProvider for tests only — it
// exists so vault tests (and anything that exercises credential storage in
// CI) never have to hit a real OS keychain, which would otherwise prompt
// interactively or simply be unavailable on a build machine.
type MemoryProvider struct {
	mu     sync.Mutex
	values map[string]string
}

// NewMemoryProvider constructs an empty in-memory provider.
func NewMemoryProvider() *MemoryProvider {
	return &MemoryProvider{values: map[string]string{}}
}

func (p *MemoryProvider) Kind() string { return "os_keychain" }

func (p *MemoryProvider) Get(id string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	v, ok := p.values[id]
	if !ok {
		return "", ErrNotFound
	}
	return v, nil
}

func (p *MemoryProvider) Set(id, value string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.values[id] = value
	return nil
}

func (p *MemoryProvider) Delete(id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.values, id)
	return nil
}
