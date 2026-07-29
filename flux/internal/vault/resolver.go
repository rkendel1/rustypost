package vault

import (
	"context"

	"flux/internal/vault/providers"
)

// SecretResolver is the only code path in the app permitted to see a
// plaintext secret value. VaultService (service.go) deliberately has no
// equivalent method, so the only way to reach a value is through here —
// and no bindings_*.go file may import this type; doing so would put a
// plaintext secret one Wails call away from the frontend.
type SecretResolver interface {
	Resolve(ctx context.Context, ref SecretReference, purpose SecretPurpose) (string, error)
}

type resolverImpl struct {
	reg       *registry
	providers map[ProviderKind]providers.CredentialProvider
}

// NewResolver constructs a SecretResolver reading metadata from the same
// baseDir a VaultService (see NewService) was constructed with, using the
// given credential backends to fetch values.
func NewResolver(baseDir string, provs ...providers.CredentialProvider) SecretResolver {
	return &resolverImpl{reg: newRegistry(baseDir), providers: indexProviders(provs)}
}

func (r *resolverImpl) Resolve(ctx context.Context, ref SecretReference, purpose SecretPurpose) (string, error) {
	meta, err := r.reg.get(ref.SecretID)
	if err != nil {
		return "", err
	}
	if !PurposeAllowed(meta.Type, purpose) {
		return "", ErrPurposeNotAllowed
	}
	p, ok := r.providers[meta.Provider]
	if !ok {
		return "", ErrBackendUnavailable
	}
	val, err := p.Get(meta.ID)
	if err != nil {
		return "", err
	}
	_ = r.reg.touchUsed(meta.ID)
	return val, nil
}
