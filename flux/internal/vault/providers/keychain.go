package providers

import (
	"errors"

	"github.com/zalando/go-keyring"
)

// keychainService is the OS keyring service name new secrets are stored
// under. It is deliberately distinct from the pre-existing "flux-crypto"
// (internal/crypto) and "reqit-github" (internal/github/auth.go) service
// names — those remain untouched for backward compatibility (see the
// vault-consolidation phase), while every new secret goes through this one,
// keyed by its own opaque secret ID rather than a caller-chosen name.
const keychainService = "reqit-vault"

// KeychainProvider stores secret values in the OS credential store: macOS
// Keychain, Windows Credential Manager, or Linux Secret Service/libsecret,
// via github.com/zalando/go-keyring's cross-platform abstraction.
type KeychainProvider struct{}

// NewKeychainProvider constructs the OS-keychain-backed provider.
func NewKeychainProvider() *KeychainProvider { return &KeychainProvider{} }

func (p *KeychainProvider) Kind() string { return "os_keychain" }

func (p *KeychainProvider) Get(id string) (string, error) {
	v, err := keyring.Get(keychainService, id)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return "", ErrNotFound
		}
		return "", err
	}
	return v, nil
}

func (p *KeychainProvider) Set(id, value string) error {
	return keyring.Set(keychainService, id, value)
}

func (p *KeychainProvider) Delete(id string) error {
	err := keyring.Delete(keychainService, id)
	if err != nil && errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}
