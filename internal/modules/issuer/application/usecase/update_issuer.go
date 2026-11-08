package usecase

import (
	"context"

	"github.com/alfredokang/kang-fiscal/internal/modules/issuer/application/command"
	"github.com/alfredokang/kang-fiscal/internal/modules/issuer/application/dto"
	"github.com/alfredokang/kang-fiscal/internal/modules/issuer/application/mapper"
	"github.com/alfredokang/kang-fiscal/internal/modules/issuer/domain/factory"
	vo "github.com/alfredokang/kang-fiscal/internal/modules/issuer/domain/valueobject"
)

// Update substitui os dados corrigíveis do emitente.
//
// Nota já emitida não muda: ela carrega o próprio snapshot do emitente (Constituição,
// Art. 2). Corrigir a razão social hoje vale da próxima nota em diante.
func (s *IssuerService) Update(ctx context.Context, cmd command.UpdateIssuer) (dto.IssuerDTO, error) {
	id, err := vo.ParseIssuerID(cmd.IssuerID)
	if err != nil {
		return dto.IssuerDTO{}, err
	}
	data, err := factory.ParseFiscalData(mapper.ToRawFiscalData(cmd.Data))
	if err != nil {
		return dto.IssuerDTO{}, err
	}
	issuer, err := s.repo.FindByID(ctx, cmd.TenantID, id)
	if err != nil {
		return dto.IssuerDTO{}, err
	}
	if err := issuer.UpdateFiscalData(data, s.now()); err != nil {
		return dto.IssuerDTO{}, err
	}
	if err := s.repo.Update(ctx, issuer); err != nil {
		return dto.IssuerDTO{}, err
	}
	return mapper.ToDTO(issuer), nil
}
