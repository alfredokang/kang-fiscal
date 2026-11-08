package usecase

import (
	"context"

	"github.com/alfredokang/kang-fiscal/internal/modules/issuer/application/command"
	"github.com/alfredokang/kang-fiscal/internal/modules/issuer/application/dto"
	"github.com/alfredokang/kang-fiscal/internal/modules/issuer/application/mapper"
	"github.com/alfredokang/kang-fiscal/internal/modules/issuer/domain"
	"github.com/alfredokang/kang-fiscal/internal/modules/issuer/domain/factory"
)

// Register cadastra o emitente, sempre em homologação.
func (s *IssuerService) Register(ctx context.Context, cmd command.RegisterIssuer) (dto.IssuerDTO, error) {
	if cmd.TenantID == "" {
		return dto.IssuerDTO{}, domain.ErrRequiredField
	}
	issuer, err := factory.NewIssuer(cmd.TenantID, cmd.CNPJ, mapper.ToRawFiscalData(cmd.Data), s.now())
	if err != nil {
		return dto.IssuerDTO{}, err
	}
	if err := s.repo.Create(ctx, issuer); err != nil {
		return dto.IssuerDTO{}, err
	}
	return mapper.ToDTO(issuer), nil
}
