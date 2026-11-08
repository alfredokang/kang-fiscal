package usecase

import (
	"context"

	"github.com/alfredokang/kang-fiscal/internal/modules/issuer/application/dto"
	"github.com/alfredokang/kang-fiscal/internal/modules/issuer/application/mapper"
	"github.com/alfredokang/kang-fiscal/internal/modules/issuer/application/query"
	vo "github.com/alfredokang/kang-fiscal/internal/modules/issuer/domain/valueobject"
)

// Get lê um emitente do tenant.
func (s *IssuerService) Get(ctx context.Context, q query.GetIssuer) (dto.IssuerDTO, error) {
	id, err := vo.ParseIssuerID(q.IssuerID)
	if err != nil {
		return dto.IssuerDTO{}, err
	}
	issuer, err := s.repo.FindByID(ctx, q.TenantID, id)
	if err != nil {
		return dto.IssuerDTO{}, err
	}
	return mapper.ToDTO(issuer), nil
}

// List lista os emitentes do tenant.
func (s *IssuerService) List(ctx context.Context, q query.ListIssuers) ([]dto.IssuerDTO, int64, error) {
	issuers, total, err := s.repo.List(ctx, q.TenantID, q.Page)
	if err != nil {
		return nil, 0, err
	}
	out := make([]dto.IssuerDTO, 0, len(issuers))
	for _, i := range issuers {
		out = append(out, mapper.ToDTO(i))
	}
	return out, total, nil
}
