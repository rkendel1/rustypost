package main

import (
	"errors"

	"flux/internal/audit"
	"flux/internal/vault"
)

// --- Vault ---
//
// These bindings expose secret metadata and lifecycle actions only. There is
// no GetSecretValue, RevealSecret, or ExportSecret method here, and there
// must never be one: Wails auto-generates a frontend binding for every
// exported method on App, so an accidental addition here would put a
// plaintext secret one IPC call away from the frontend. Anything that needs
// a resolved value (e.g. an Integration authenticating to GitHub) must do so
// from backend code using vault.SecretResolver directly — never through a
// bindings_*.go method.

func (a *App) ListSecrets(projectID string) ([]vault.SecretMetadata, error) {
	if a.vaultSvc == nil {
		return nil, errors.New("vault not initialised")
	}
	return a.vaultSvc.ListSecrets(a.ctx, projectID)
}

func (a *App) StoreSecret(input vault.StoreSecretInput) (vault.SecretReference, error) {
	if a.vaultSvc == nil {
		return vault.SecretReference{}, errors.New("vault not initialised")
	}
	ref, err := a.vaultSvc.StoreSecret(a.ctx, input)
	if err != nil {
		return vault.SecretReference{}, err
	}
	if a.audit != nil {
		_ = a.audit.Log("user", audit.ActionCreate, "secret", ref.SecretID, "", map[string]string{"name": input.Name})
	}
	return ref, nil
}

func (a *App) DeleteSecret(secretID string) error {
	if a.vaultSvc == nil {
		return errors.New("vault not initialised")
	}
	if err := a.vaultSvc.DeleteSecret(a.ctx, vault.SecretReference{SecretID: secretID}); err != nil {
		return err
	}
	if a.audit != nil {
		_ = a.audit.Log("user", audit.ActionDelete, "secret", secretID, "", nil)
	}
	return nil
}

func (a *App) RotateSecret(secretID, newValue string) error {
	if a.vaultSvc == nil {
		return errors.New("vault not initialised")
	}
	return a.vaultSvc.Rotate(a.ctx, vault.SecretReference{SecretID: secretID}, newValue)
}
