package server

import (
	"context"
	"errors"

	certout "github.com/alfredokang/kang-fiscal/internal/modules/certificate/application/port/output"
	certdomain "github.com/alfredokang/kang-fiscal/internal/modules/certificate/domain"
	issuerquery "github.com/alfredokang/kang-fiscal/internal/modules/issuer/application/query"
	issuerusecase "github.com/alfredokang/kang-fiscal/internal/modules/issuer/application/usecase"
	issuerdomain "github.com/alfredokang/kang-fiscal/internal/modules/issuer/domain"
)

// certificateIssuerAdapter implementa a porta IssuerReader do módulo certificate sobre o
// módulo issuer. É aqui, e só aqui, que os dois se enxergam (Constituição, Art. 1.2).
type certificateIssuerAdapter struct {
	issuers *issuerusecase.IssuerService
}

func (a certificateIssuerAdapter) IssuerForCertificate(ctx context.Context, tenantID, issuerRef string) (certout.IssuerSnapshot, error) {
	d, err := a.issuers.Get(ctx, issuerquery.GetIssuer{TenantID: tenantID, IssuerID: issuerRef})
	// O erro do emitente vira o erro do certificado: o módulo certificate não conhece os
	// erros do issuer, e o handler dele só sabe traduzir os próprios.
	if errors.Is(err, issuerdomain.ErrIssuerNotFound) || errors.Is(err, issuerdomain.ErrInvalidIssuerID) {
		return certout.IssuerSnapshot{}, certdomain.ErrIssuerNotFound
	}
	if err != nil {
		return certout.IssuerSnapshot{}, err
	}
	return certout.IssuerSnapshot{CNPJ: d.CNPJ}, nil
}
