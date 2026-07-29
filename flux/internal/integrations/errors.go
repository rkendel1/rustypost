package integrations

import "errors"

var (
	ErrIntegrationNotFound = errors.New("integrations: integration not found")
	ErrNameRequired        = errors.New("integrations: name is required")
	ErrProviderRequired    = errors.New("integrations: provider is required")
)
