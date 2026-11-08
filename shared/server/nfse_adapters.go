package server

import (
	"context"
	"errors"

	certcmd "github.com/alfredokang/kang-fiscal/internal/modules/certificate/application/command"
	certusecase "github.com/alfredokang/kang-fiscal/internal/modules/certificate/application/usecase"
	certdomain "github.com/alfredokang/kang-fiscal/internal/modules/certificate/domain"
	issuerquery "github.com/alfredokang/kang-fiscal/internal/modules/issuer/application/query"
	issuerusecase "github.com/alfredokang/kang-fiscal/internal/modules/issuer/application/usecase"
	issuerdomain "github.com/alfredokang/kang-fiscal/internal/modules/issuer/domain"
	nfseout "github.com/alfredokang/kang-fiscal/internal/modules/nfse/application/port/output"
	nfsedomain "github.com/alfredokang/kang-fiscal/internal/modules/nfse/domain"
	numberingusecase "github.com/alfredokang/kang-fiscal/internal/modules/numbering/application/usecase"
	numberingdomain "github.com/alfredokang/kang-fiscal/internal/modules/numbering/domain"
	"github.com/alfredokang/kang-fiscal/internal/modules/numbering/domain/entity"
	"github.com/alfredokang/kang-fiscal/pkg/certificate"
)

// nfseAdapter implementa as portas da NFS-e sobre o issuer, o certificate e o numbering.
type nfseAdapter struct {
	issuers *issuerusecase.IssuerService
	certs   *certusecase.CertificateService
	numbers *numberingusecase.NumberingService
}

func (a nfseAdapter) IssuerForNFSe(ctx context.Context, tenantID, issuerRef string) (nfseout.Issuer, error) {
	d, err := a.issuers.Get(ctx, issuerquery.GetIssuer{TenantID: tenantID, IssuerID: issuerRef})
	if errors.Is(err, issuerdomain.ErrIssuerNotFound) || errors.Is(err, issuerdomain.ErrInvalidIssuerID) {
		return nfseout.Issuer{}, nfsedomain.ErrIssuerNotFound
	}
	if err != nil {
		return nfseout.Issuer{}, err
	}
	return nfseout.Issuer{CNPJ: d.CNPJ, MunicipalRegistration: d.MunicipalRegistration, CityCode: d.Address.CityCode,
		UF: d.Address.UF, Environment: d.Environment, CRT: d.TaxRegime}, nil
}

func (a nfseAdapter) SignerForNFSe(ctx context.Context, tenantID, issuerRef string) (*certificate.Certificate, error) {
	c, err := a.certs.Signer(ctx, certcmd.IssuerCertificate{TenantID: tenantID, IssuerRef: issuerRef})
	if errors.Is(err, certdomain.ErrNotFound) || errors.Is(err, certdomain.ErrExpired) || errors.Is(err, certdomain.ErrInvalidID) {
		return nil, nfsedomain.ErrNoCertificate
	}
	return c, err
}

func (a nfseAdapter) ReserveDPS(ctx context.Context, tenantID, issuerRef, env string, series int, docRef string) (string, int, error) {
	r, err := a.numbers.Reserve(ctx, entity.SeriesKey{TenantID: tenantID, IssuerRef: issuerRef, Model: 99, Series: series,
		Environment: env}, docRef)
	return r.ID, r.Number, err
}

func (a nfseAdapter) CloseDPS(ctx context.Context, tenantID, ref string) error {
	if err := a.numbers.MarkUsed(ctx, tenantID, ref); err != nil && !errors.Is(err, numberingdomain.ErrNotFound) {
		return err
	}
	return nil
}
