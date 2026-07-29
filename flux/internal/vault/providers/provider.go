// Package providers implements the credential backends the vault can store
// secret values in. A CredentialProvider only ever sees an opaque,
// vault-generated secret ID and a plaintext value — it has no knowledge of
// what kind of secret it holds, what project owns it, or what it's used for.
package providers

import "errors"

// ErrNotFound is returned when no credential exists for a given secret ID.
var ErrNotFound = errors.New("providers: credential not found")

// CredentialProvider stores and retrieves secret values keyed by an opaque
// vault-generated secret ID.
type CredentialProvider interface {
	// Kind identifies this backend (e.g. "os_keychain", "encrypted_file").
	Kind() string
	Get(id string) (string, error)
	Set(id, value string) error
	Delete(id string) error
}
