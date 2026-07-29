package vault

import (
	"context"
	"time"

	"github.com/google/uuid"

	"flux/internal/vault/providers"
)

// StoreSecretInput describes a new secret to store, or a value to store
// under an explicitly chosen provider.
type StoreSecretInput struct {
	ProjectID   string
	Name        string
	Description string
	Type        SecretType
	Scope       SecretScope
	Value       string
	// Provider optionally pins which backend to use. If empty, the service
	// prefers the OS keychain, then the encrypted-file fallback (if one was
	// registered — see providers.NewEncryptedFileProvider) — and returns
	// ErrBackendUnavailable rather than ever storing a value in plaintext.
	Provider ProviderKind
}

// VaultService is the metadata-and-lifecycle surface the Wails binding
// layer is allowed to call. It deliberately has no method that returns a
// secret value — only SecretResolver (resolver.go) can do that, and nothing
// in bindings_*.go may import it.
type VaultService interface {
	ListSecrets(ctx context.Context, projectID string) ([]SecretMetadata, error)
	StoreSecret(ctx context.Context, input StoreSecretInput) (SecretReference, error)
	DeleteSecret(ctx context.Context, ref SecretReference) error
	Rotate(ctx context.Context, ref SecretReference, newValue string) error
}

type service struct {
	reg       *registry
	providers map[ProviderKind]providers.CredentialProvider
}

// NewService constructs the default VaultService. baseDir is where secret
// metadata (never values) is persisted. provs are the credential backends
// available to this instance — production wiring registers the OS keychain
// unconditionally and the encrypted-file fallback only after the user has
// explicitly consented to it.
func NewService(baseDir string, provs ...providers.CredentialProvider) VaultService {
	return &service{reg: newRegistry(baseDir), providers: indexProviders(provs)}
}

func indexProviders(provs []providers.CredentialProvider) map[ProviderKind]providers.CredentialProvider {
	m := make(map[ProviderKind]providers.CredentialProvider, len(provs))
	for _, p := range provs {
		m[ProviderKind(p.Kind())] = p
	}
	return m
}

func (s *service) resolveProvider(preferred ProviderKind) (ProviderKind, providers.CredentialProvider, error) {
	if preferred != "" {
		p, ok := s.providers[preferred]
		if !ok {
			return "", nil, ErrBackendUnavailable
		}
		return preferred, p, nil
	}
	if p, ok := s.providers[ProviderOSKeychain]; ok {
		return ProviderOSKeychain, p, nil
	}
	if p, ok := s.providers[ProviderEncryptedFile]; ok {
		return ProviderEncryptedFile, p, nil
	}
	return "", nil, ErrBackendUnavailable
}

func (s *service) ListSecrets(ctx context.Context, projectID string) ([]SecretMetadata, error) {
	return s.reg.list(projectID)
}

func (s *service) StoreSecret(ctx context.Context, input StoreSecretInput) (SecretReference, error) {
	if input.Name == "" {
		return SecretReference{}, ErrNameRequired
	}
	if input.Value == "" {
		return SecretReference{}, ErrValueRequired
	}
	kind, p, err := s.resolveProvider(input.Provider)
	if err != nil {
		return SecretReference{}, err
	}

	id := uuid.NewString()
	if err := p.Set(id, input.Value); err != nil {
		return SecretReference{}, err
	}
	ts := time.Now().UTC().Format(time.RFC3339)
	meta := SecretMetadata{
		ID:          id,
		ProjectID:   input.ProjectID,
		Name:        input.Name,
		Description: input.Description,
		Type:        input.Type,
		Provider:    kind,
		Scope:       input.Scope,
		Status:      SecretStatusConfigured,
		CreatedAt:   ts,
		UpdatedAt:   ts,
	}
	if err := s.reg.put(meta); err != nil {
		_ = p.Delete(id) // best-effort: don't leave an orphaned credential behind
		return SecretReference{}, err
	}
	return SecretReference{SecretID: id}, nil
}

func (s *service) DeleteSecret(ctx context.Context, ref SecretReference) error {
	meta, err := s.reg.get(ref.SecretID)
	if err != nil {
		return err
	}
	if p, ok := s.providers[meta.Provider]; ok {
		if err := p.Delete(meta.ID); err != nil {
			return err
		}
	}
	return s.reg.remove(meta.ID)
}

func (s *service) Rotate(ctx context.Context, ref SecretReference, newValue string) error {
	if newValue == "" {
		return ErrValueRequired
	}
	meta, err := s.reg.get(ref.SecretID)
	if err != nil {
		return err
	}
	p, ok := s.providers[meta.Provider]
	if !ok {
		return ErrBackendUnavailable
	}
	if err := p.Set(meta.ID, newValue); err != nil {
		return err
	}
	meta.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return s.reg.put(meta)
}
