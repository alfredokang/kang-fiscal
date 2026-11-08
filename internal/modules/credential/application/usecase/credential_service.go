// Package usecase orquestra as credenciais.
package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/alfredokang/kang-fiscal/internal/modules/credential/application/port/output"
	"github.com/alfredokang/kang-fiscal/internal/modules/credential/domain"
	"github.com/alfredokang/kang-fiscal/internal/modules/credential/domain/entity"
)

// CredentialService é a fachada do módulo.
type CredentialService struct {
	repo   output.KeyRepository
	pepper []byte
	now    func() time.Time
}

// NewCredentialService monta o serviço com o pepper do ambiente.
func NewCredentialService(repo output.KeyRepository, pepper string) *CredentialService {
	return &CredentialService{repo: repo, pepper: []byte(pepper), now: func() time.Time { return time.Now().UTC() }}
}

// Create cria a credencial e devolve o token — a única vez em que ele aparece.
func (s *CredentialService) Create(ctx context.Context, tenantID, name string, scopes []string) (*entity.Key, string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return nil, "", err
	}
	k, token, err := entity.NewKey(id.String(), tenantID, name, scopes, s.pepper, s.now())
	if err != nil {
		return nil, "", err
	}
	return k, token, s.repo.Create(ctx, k)
}

// List lista as credenciais do tenant, sem segredo.
func (s *CredentialService) List(ctx context.Context, tenantID string) ([]*entity.Key, error) {
	return s.repo.List(ctx, tenantID)
}

// Revoke revoga a credencial: a partir daqui ela não autentica mais nada.
func (s *CredentialService) Revoke(ctx context.Context, id string) error {
	return s.repo.Revoke(ctx, id, s.now())
}

// Resolve confere o token e devolve a credencial. Qualquer falha — formato, prefixo
// desconhecido, segredo errado, revogada — é o mesmo erro: dizer qual parte errou ajuda
// quem está adivinhando.
func (s *CredentialService) Resolve(ctx context.Context, token string) (*entity.Key, error) {
	if len(s.pepper) == 0 {
		return nil, domain.ErrNoPepper
	}
	prefix, secret, ok := entity.Split(token)
	if !ok {
		return nil, domain.ErrUnauthorized
	}
	k, err := s.repo.FindByPrefix(ctx, prefix)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, domain.ErrUnauthorized
	}
	if err != nil {
		return nil, err
	}
	if !k.Matches(s.pepper, secret) || k.RevokedAt != nil {
		return nil, domain.ErrUnauthorized
	}
	_ = s.repo.Touch(ctx, k.ID, s.now())
	return k, nil
}
