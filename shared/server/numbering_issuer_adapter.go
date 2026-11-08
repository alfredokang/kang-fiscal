package server

import (
	"context"
	"errors"

	issuerquery "github.com/alfredokang/kang-fiscal/internal/modules/issuer/application/query"
	issuerusecase "github.com/alfredokang/kang-fiscal/internal/modules/issuer/application/usecase"
	issuerdomain "github.com/alfredokang/kang-fiscal/internal/modules/issuer/domain"
)

// numberingIssuerAdapter implementa a porta IssuerChecker da numeração sobre o issuer.
type numberingIssuerAdapter struct {
	issuers *issuerusecase.IssuerService
}

func (a numberingIssuerAdapter) IssuerExists(ctx context.Context, tenantID, issuerRef string) (bool, error) {
	_, err := a.issuers.Get(ctx, issuerquery.GetIssuer{TenantID: tenantID, IssuerID: issuerRef})
	if errors.Is(err, issuerdomain.ErrIssuerNotFound) || errors.Is(err, issuerdomain.ErrInvalidIssuerID) {
		return false, nil
	}
	return err == nil, err
}
