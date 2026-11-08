package server

import (
	"context"
	"errors"

	certcmd "github.com/alfredokang/kang-fiscal/internal/modules/certificate/application/command"
	certusecase "github.com/alfredokang/kang-fiscal/internal/modules/certificate/application/usecase"
	certdomain "github.com/alfredokang/kang-fiscal/internal/modules/certificate/domain"
	dfeout "github.com/alfredokang/kang-fiscal/internal/modules/dfe/application/port/output"
	dfedomain "github.com/alfredokang/kang-fiscal/internal/modules/dfe/domain"
	issuerquery "github.com/alfredokang/kang-fiscal/internal/modules/issuer/application/query"
	issuerusecase "github.com/alfredokang/kang-fiscal/internal/modules/issuer/application/usecase"
	issuerdomain "github.com/alfredokang/kang-fiscal/internal/modules/issuer/domain"
	"github.com/alfredokang/kang-fiscal/pkg/certificate"
)

// dfeAdapter implementa as portas do recebimento sobre o issuer e o certificate.
type dfeAdapter struct {
	issuers *issuerusecase.IssuerService
	certs   *certusecase.CertificateService
}

func (a dfeAdapter) IssuerForDFe(ctx context.Context, tenantID, issuerRef string) (dfeout.Issuer, error) {
	d, err := a.issuers.Get(ctx, issuerquery.GetIssuer{TenantID: tenantID, IssuerID: issuerRef})
	if errors.Is(err, issuerdomain.ErrIssuerNotFound) || errors.Is(err, issuerdomain.ErrInvalidIssuerID) {
		return dfeout.Issuer{}, dfedomain.ErrIssuerNotFound
	}
	if err != nil {
		return dfeout.Issuer{}, err
	}
	return dfeout.Issuer{CNPJ: d.CNPJ, UF: d.Address.UF, Environment: d.Environment}, nil
}

func (a dfeAdapter) SignerForDFe(ctx context.Context, tenantID, issuerRef string) (*certificate.Certificate, error) {
	c, err := a.certs.Signer(ctx, certcmd.IssuerCertificate{TenantID: tenantID, IssuerRef: issuerRef})
	if errors.Is(err, certdomain.ErrNotFound) || errors.Is(err, certdomain.ErrExpired) || errors.Is(err, certdomain.ErrInvalidID) {
		return nil, dfedomain.ErrNoCertificate
	}
	return c, err
}
