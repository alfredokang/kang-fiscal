package usecase

import (
	"context"

	"github.com/alfredokang/kang-fiscal/internal/modules/issuer/application/command"
	"github.com/alfredokang/kang-fiscal/internal/modules/issuer/application/dto"
	"github.com/alfredokang/kang-fiscal/internal/modules/issuer/application/mapper"
	vo "github.com/alfredokang/kang-fiscal/internal/modules/issuer/domain/valueobject"
)

// Promote passa o emitente para produção. A regra (cadastro completo, sem volta) é do
// agregado; aqui só se carrega e se grava.
func (s *IssuerService) Promote(ctx context.Context, cmd command.PromoteIssuer) (dto.IssuerDTO, error) {
	id, err := vo.ParseIssuerID(cmd.IssuerID)
	if err != nil {
		return dto.IssuerDTO{}, err
	}
	issuer, err := s.repo.FindByID(ctx, cmd.TenantID, id)
	if err != nil {
		return dto.IssuerDTO{}, err
	}
	if err := issuer.PromoteToProduction(s.now()); err != nil {
		return dto.IssuerDTO{}, err
	}
	if err := s.repo.Update(ctx, issuer); err != nil {
		return dto.IssuerDTO{}, err
	}
	return mapper.ToDTO(issuer), nil
}
