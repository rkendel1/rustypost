package integrations

import (
	"context"
	"time"

	"github.com/google/uuid"

	"flux/internal/vault"
)

// CreateIntegrationInput describes a new Integration to configure.
type CreateIntegrationInput struct {
	ProjectID  string
	Provider   ProviderID
	Name       string
	Credential vault.SecretReference
}

// IntegrationService owns Integration lifecycle. It never handles secret
// values directly — credentials are always vault.SecretReferences supplied
// by the caller (typically after the caller stored a value via
// vault.VaultService.StoreSecret).
type IntegrationService interface {
	List(ctx context.Context, projectID string) ([]Integration, error)
	Create(ctx context.Context, input CreateIntegrationInput) (*Integration, error)
	Remove(ctx context.Context, integrationID string) error
	Validate(ctx context.Context, integrationID string) (IntegrationHealth, error)
}

type service struct {
	reg       *registry
	providers map[ProviderID]IntegrationProvider
	resolver  vault.SecretResolver
}

// NewService constructs the default IntegrationService. baseDir is where
// Integration records (never secret values) are persisted. providers are
// the registered IntegrationProvider implementations (GitHub, Local Git,
// Generic HTTP, ...); resolver is used only inside Validate, to check a
// credential actually works — it is never exposed back to callers.
func NewService(baseDir string, resolver vault.SecretResolver, provs ...IntegrationProvider) IntegrationService {
	m := make(map[ProviderID]IntegrationProvider, len(provs))
	for _, p := range provs {
		m[p.ID()] = p
	}
	return &service{reg: newRegistry(baseDir), providers: m, resolver: resolver}
}

func (s *service) List(ctx context.Context, projectID string) ([]Integration, error) {
	return s.reg.list(projectID)
}

func (s *service) Create(ctx context.Context, input CreateIntegrationInput) (*Integration, error) {
	if input.Name == "" {
		return nil, ErrNameRequired
	}
	if input.Provider == "" {
		return nil, ErrProviderRequired
	}
	ts := time.Now().UTC().Format(time.RFC3339)
	in := Integration{
		ID:         uuid.NewString(),
		ProjectID:  input.ProjectID,
		Provider:   input.Provider,
		Name:       input.Name,
		Status:     IntegrationStatusRequiresSetup,
		Credential: input.Credential,
		CreatedAt:  ts,
		UpdatedAt:  ts,
	}
	if input.Credential.SecretID != "" {
		in.Status = IntegrationStatusConnected
	}
	if err := s.reg.put(in); err != nil {
		return nil, err
	}
	return &in, nil
}

// Remove deletes the Integration record only. It deliberately never deletes
// the underlying vault secret: a credential may be an application-scoped
// secret shared by other integrations or projects, and secret deletion must
// always be its own explicit, confirmed action (see vault.VaultService.DeleteSecret).
func (s *service) Remove(ctx context.Context, integrationID string) error {
	return s.reg.remove(integrationID)
}

func (s *service) Validate(ctx context.Context, integrationID string) (IntegrationHealth, error) {
	in, err := s.reg.get(integrationID)
	if err != nil {
		return IntegrationHealth{}, err
	}
	provider, ok := s.providers[in.Provider]
	if !ok {
		return IntegrationHealth{Status: IntegrationStatusRequiresSetup, Detail: "no provider registered for " + string(in.Provider)}, nil
	}
	health, err := provider.Validate(ctx, in, s.resolver)
	if err != nil {
		return IntegrationHealth{Status: IntegrationStatusError, Detail: err.Error()}, nil
	}
	return health, nil
}
