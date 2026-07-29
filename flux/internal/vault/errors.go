package vault

import "errors"

var (
	ErrSecretNotFound     = errors.New("vault: secret not found")
	ErrNameRequired       = errors.New("vault: secret name is required")
	ErrValueRequired      = errors.New("vault: secret value is required")
	ErrPurposeNotAllowed  = errors.New("vault: secret type is not allowed for the requested purpose")
	ErrBackendUnavailable = errors.New("vault: no credential backend is available")
)
